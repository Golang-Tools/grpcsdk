package grpcsdk

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// countingService 统计调用次数的测试服务,可以模拟调用延迟与固定错误
type countingService struct {
	calls    int32
	delay    time.Duration
	err      error
	lastAuth atomic.Value
}

// count 返回被调用的次数
// @returns int32 调用次数
func (s *countingService) count() int32 {
	return atomic.LoadInt32(&s.calls)
}

// auth 返回最后一次请求携带的authorization
// @returns string authorization值
func (s *countingService) auth() string {
	v, _ := s.lastAuth.Load().(string)
	return v
}

// Do 记录调用次数后按配置返回结果
func (s *countingService) Do(ctx context.Context, in *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	atomic.AddInt32(&s.calls, 1)
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if v := md.Get("authorization"); len(v) > 0 {
			s.lastAuth.Store(v[0])
		}
	}
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if s.err != nil {
		return nil, s.err
	}
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

// counterServer 计数服务的接口
type counterServer interface {
	Do(context.Context, *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error)
}

// countingDoHandler 计数服务Do方法的handler
func countingDoHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(healthpb.HealthCheckRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(counterServer).Do(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: countingFullMethod,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(counterServer).Do(ctx, req.(*healthpb.HealthCheckRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// countingFullMethod 计数服务的方法全名
const countingFullMethod = "/grpcsdk.test.Counting/Do"

// countingServiceDesc 计数服务的描述对象
var countingServiceDesc = grpc.ServiceDesc{
	ServiceName: "grpcsdk.test.Counting",
	HandlerType: (*counterServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Do",
			Handler:    countingDoHandler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "counting.proto",
}

// startCountingServer 启动一个计数服务的测试服务器
// @params t *testing.T 测试对象
// @params svc *countingService 计数服务
// @returns string 服务地址
// @returns func() 关闭服务器的清理函数
func startCountingServer(t *testing.T, svc *countingService) (string, func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test server error: %v", err)
	}
	s := grpc.NewServer()
	s.RegisterService(&countingServiceDesc, svc)
	go func() {
		_ = s.Serve(lis)
	}()
	return lis.Addr().String(), func() {
		s.Stop()
		_ = lis.Close()
	}
}

// invokeCounting 调用计数服务的Do方法
// @params conn GrpcConnInterface 连接对象
// @returns error 错误信息
func invokeCounting(conn GrpcConnInterface) error {
	var resp healthpb.HealthCheckResponse
	return conn.Invoke(context.Background(), countingFullMethod, &healthpb.HealthCheckRequest{}, &resp)
}

// TestSDKRetryPolicy 验证重试策略会按maxAttempts重试
func TestSDKRetryPolicy(t *testing.T) {
	svc := &countingService{err: status.Error(codes.Unavailable, "retry me")}
	addr, cleanup := startCountingServer(t, svc)
	defer cleanup()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(
		WithQueryAddresses(addr),
		WithRetryPolicy(&RetryPolicy{
			MaxAttempts:          3,
			InitialBackoff:       10 * time.Millisecond,
			MaxBackoff:           50 * time.Millisecond,
			BackoffMultiplier:    2,
			RetryableStatusCodes: []string{"UNAVAILABLE"},
		}),
	); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()

	clientgetter, err := sdk.NewClientGetter()
	if err != nil {
		t.Fatalf("NewClientGetter error: %v", err)
	}
	client, err := clientgetter.Acquire()
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	defer clientgetter.Release(client)
	if err := invokeCounting(client.GetConn()); err == nil {
		t.Fatal("服务端一直返回Unavailable,调用应该失败")
	}
	if got := svc.count(); got != 3 {
		t.Fatalf("重试策略未生效,服务端应该被调用3次, got: %v", got)
	}
}

// TestSDKRetryPolicyValidation 验证重试策略的参数校验
func TestSDKRetryPolicyValidation(t *testing.T) {
	cases := []struct {
		name   string
		policy *RetryPolicy
	}{
		{"maxAttempts小于2", &RetryPolicy{MaxAttempts: 1, RetryableStatusCodes: []string{"UNAVAILABLE"}}},
		{"maxAttempts大于5", &RetryPolicy{MaxAttempts: 6, RetryableStatusCodes: []string{"UNAVAILABLE"}}},
		{"状态码列表为空", &RetryPolicy{MaxAttempts: 2}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
			err := sdk.Init(WithQueryAddresses("127.0.0.1:1"), WithRetryPolicy(c.policy))
			if !errors.Is(err, ErrInvalidRetryPolicy) {
				t.Fatalf("期待ErrInvalidRetryPolicy, got: %v", err)
			}
		})
	}
}

// TestRetryPolicyToMethodConfig 验证重试策略会被转换为methodConfig格式
func TestRetryPolicyToMethodConfig(t *testing.T) {
	p := &RetryPolicy{
		MaxAttempts:          3,
		InitialBackoff:       10 * time.Millisecond,
		MaxBackoff:           50 * time.Millisecond,
		BackoffMultiplier:    2,
		RetryableStatusCodes: []string{"UNAVAILABLE"},
	}
	mc := p.toMethodConfig()
	names, ok := mc["name"].([]interface{})
	if !ok || len(names) != 1 {
		t.Fatalf("默认methodConfig应该包含一个空name, got: %v", mc["name"])
	}
	rp, ok := mc["retryPolicy"].(map[string]interface{})
	if !ok {
		t.Fatalf("methodConfig中应该包含retryPolicy, got: %v", mc)
	}
	if rp["initialBackoff"] != "0.01s" || rp["maxBackoff"] != "0.05s" {
		t.Fatalf("退避时长格式不正确, got: %v, %v", rp["initialBackoff"], rp["maxBackoff"])
	}
	if rp["maxAttempts"] != 3 || rp["backoffMultiplier"] != float64(2) {
		t.Fatalf("重试参数不正确, got: %v", rp)
	}
}

// TestSDKLoadBalancingPolicy 验证多地址下可以指定负载均衡策略
func TestSDKLoadBalancingPolicy(t *testing.T) {
	serviceName := "testapp-1_2_3"
	addr1, cleanup1 := startHealthServerWithServices(t, []string{serviceName})
	defer cleanup1()
	addr2, cleanup2 := startHealthServerWithServices(t, []string{serviceName})
	defer cleanup2()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(
		WithQueryAddresses(addr1, addr2),
		WithLoadBalancingPolicy("least_request"),
		WithRequesterAppName("testapp"),
		WithRequesterAppVersion("1.2.3"),
	); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()
	if got := sdk.serviceconfig["loadBalancingPolicy"]; got != "least_request" {
		t.Fatalf("负载均衡策略应该使用指定的策略, got: %v", got)
	}
	cli, release := sdk.GetClient()
	defer release()
	for i := 0; i < 4; i++ {
		if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
			t.Fatalf("Check error: %v", err)
		}
	}
}

// TestSDKLoadBalancingPolicySingleAddress 验证单地址下也可以显式指定负载均衡策略
func TestSDKLoadBalancingPolicySingleAddress(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(WithQueryAddresses(addr), WithLoadBalancingPolicy("round_robin")); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()
	if got := sdk.serviceconfig["loadBalancingPolicy"]; got != "round_robin" {
		t.Fatalf("单地址下显式指定策略时配置不正确, got: %v", got)
	}
	cli, release := sdk.GetClient()
	defer release()
	if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
}

// TestSDKConnectParamsAndIdleTimeout 验证连接参数与空闲回收设置
func TestSDKConnectParamsAndIdleTimeout(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(
		WithQueryAddresses(addr),
		WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  100 * time.Millisecond,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   5 * time.Second,
			},
			MinConnectTimeout: time.Second,
		}),
		WithIdleTimeoutMS(60000),
	); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()
	cli, release := sdk.GetClient()
	defer release()
	if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}

	// 负数表示禁用grpc默认的空闲回收
	sdk2 := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk2.Init(WithQueryAddresses(addr), WithIdleTimeoutMS(-1)); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk2.Close()
	cli2, release2 := sdk2.GetClient()
	defer release2()
	if _, err := cli2.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
}
