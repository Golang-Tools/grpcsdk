package grpcsdk

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
)

// countingStatsHandler 统计TagRPC事件的测试用stats.Handler
type countingStatsHandler struct {
	tagrpc int32
}

// TagRPC 统计RPC开始事件
func (h *countingStatsHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	atomic.AddInt32(&h.tagrpc, 1)
	return ctx
}

// HandleRPC 处理RPC事件
func (h *countingStatsHandler) HandleRPC(ctx context.Context, rs stats.RPCStats) {}

// TagConn 统计连接事件
func (h *countingStatsHandler) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	return ctx
}

// HandleConn 处理连接事件
func (h *countingStatsHandler) HandleConn(ctx context.Context, cs stats.ConnStats) {}

// testPerRPCCredentials 测试用的请求级凭证
type testPerRPCCredentials struct {
	token string
}

// GetRequestMetadata 返回请求携带的元数据
func (c *testPerRPCCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + c.token}, nil
}

// RequireTransportSecurity 返回是否需要安全传输,测试中使用insecure连接因此返回false
func (c *testPerRPCCredentials) RequireTransportSecurity() bool {
	return false
}

// TestSDKAdditionalDialOptions 验证可以通过额外选项透传任意grpc连接选项
func TestSDKAdditionalDialOptions(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()

	var mu sync.Mutex
	var calls int
	interceptor := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		mu.Lock()
		calls++
		mu.Unlock()
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(
		WithQueryAddresses(addr),
		WithAdditionalDialOptions(grpc.WithChainUnaryInterceptor(interceptor)),
	); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()
	cli, release := sdk.GetClient()
	defer release()
	if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls == 0 {
		t.Fatal("透传的拦截器没有生效")
	}
}

// TestSDKStatsHandler 验证stats.Handler可以被正确接入
func TestSDKStatsHandler(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()

	h := &countingStatsHandler{}
	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(WithQueryAddresses(addr), WithStatsHandlers(h)); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()
	cli, release := sdk.GetClient()
	defer release()
	if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if got := atomic.LoadInt32(&h.tagrpc); got == 0 {
		t.Fatal("stats.Handler没有收到RPC事件")
	}
}

// TestSDKPerRPCCredentials 验证请求级凭证会携带到服务端
func TestSDKPerRPCCredentials(t *testing.T) {
	svc := &countingService{}
	addr, cleanup := startCountingServer(t, svc)
	defer cleanup()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(
		WithQueryAddresses(addr),
		WithPerRPCCredentials(&testPerRPCCredentials{token: "test-token"}),
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
	if err := invokeCounting(client.GetConn()); err != nil {
		t.Fatalf("Do error: %v", err)
	}
	if got := svc.auth(); got != "Bearer test-token" {
		t.Fatalf("服务端没有收到预期的凭证, got: %q", got)
	}
}

// TestSDKAuthority 验证设置authority后连接与请求正常
func TestSDKAuthority(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(WithQueryAddresses(addr), WithAuthority("grpc.test.example")); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()
	cli, release := sdk.GetClient()
	defer release()
	if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
}

// TestSDKWaitForReady 验证开启等待就绪后请求会等待到超时而不是快速失败
func TestSDKWaitForReady(t *testing.T) {
	addr := unusedAddr(t)

	// 默认情况下连接不可用时快速失败
	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(WithQueryAddresses(addr)); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()
	cli, release := sdk.GetClient()
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := cli.Check(ctx, &healthpb.HealthCheckRequest{}); status.Code(err) != codes.Unavailable {
		t.Fatalf("默认情况下连接不可用时应该快速失败, got: %v", err)
	}

	// 开启等待就绪后请求会阻塞直到超时
	sdk2 := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk2.Init(WithQueryAddresses(addr), WithWaitForReady()); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk2.Close()
	cli2, release2 := sdk2.GetClient()
	defer release2()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel2()
	if _, err := cli2.Check(ctx2, &healthpb.HealthCheckRequest{}); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("等待就绪时应该等到超时, got: %v", err)
	}
}
