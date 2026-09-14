package grpcsdk

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/Golang-Tools/optparams"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
)

// TestSDKInitGetClient 验证SDK的初始化、获取客户端、发起请求与关闭
func TestSDKInitGetClient(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()

	for _, withpool := range []bool{false, true} {
		name := "单客户端"
		if withpool {
			name = "客户端池"
		}
		t.Run(name, func(t *testing.T) {
			sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
			opts := []optparams.Option[SDKConfig]{WithQueryAddresses(addr)}
			if withpool {
				opts = append(opts,
					WithClientPool(),
					WithClientPoolReservations(1),
					WithClientPoolLimits(2),
					WithClientPoolAcquireWaitTime(200),
				)
			}
			if err := sdk.Init(opts...); err != nil {
				t.Fatalf("Init error: %v", err)
			}
			defer sdk.Close()

			cli, release := sdk.GetClient()
			if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
				t.Fatalf("Check error: %v", err)
			}
			release()
		})
	}
}

// TestSDKGetClientForce 验证池模式下使用Force获取客户端
func TestSDKGetClientForce(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(
		WithQueryAddresses(addr),
		WithClientPool(),
		WithClientPoolReservations(1),
		WithClientPoolLimits(1),
		WithClientPoolAcquireWaitTime(100),
	); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()

	cli1, release1 := sdk.GetClient()
	defer release1()
	cli2, release2 := sdk.GetClient(Force())
	defer release2()
	if _, err := cli1.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if _, err := cli2.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
}

// TestSDKInitIdempotent 验证重复调用Init会重建客户端获取器并关闭旧的池
func TestSDKInitIdempotent(t *testing.T) {
	addr1, cleanup1 := startHealthServer(t)
	defer cleanup1()
	addr2, cleanup2 := startHealthServer(t)
	defer cleanup2()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(WithQueryAddresses(addr1), WithClientPool()); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	first, err := sdk.NewClientGetter()
	if err != nil {
		t.Fatalf("NewClientGetter error: %v", err)
	}
	if err := sdk.Init(WithQueryAddresses(addr2), WithClientPool()); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	second, err := sdk.NewClientGetter()
	if err != nil {
		t.Fatalf("NewClientGetter error: %v", err)
	}
	if first == second {
		t.Fatal("重复Init后应该重建客户端获取器")
	}
	oldpool, ok := first.(*GrpcConnPool[healthpb.HealthClient])
	if !ok {
		t.Fatalf("客户端获取器类型不正确: %T", first)
	}
	if !oldpool.IsClosed() {
		t.Fatal("重复Init后旧的池应该被关闭")
	}

	// 新的池可以正常使用
	cli, release := sdk.GetClient()
	defer release()
	if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if err := sdk.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

// TestSDKNewClientGetterIdempotent 验证重复调用NewClientGetter返回同一个获取器
func TestSDKNewClientGetterIdempotent(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(WithQueryAddresses(addr)); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()

	first, err := sdk.NewClientGetter()
	if err != nil {
		t.Fatalf("NewClientGetter error: %v", err)
	}
	second, err := sdk.NewClientGetter()
	if err != nil {
		t.Fatalf("NewClientGetter error: %v", err)
	}
	if first != second {
		t.Fatal("重复调用NewClientGetter应该返回同一个获取器")
	}
}

// TestSDKCloseAndReopen 验证关闭SDK后再次获取客户端会重新建立连接
func TestSDKCloseAndReopen(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(WithQueryAddresses(addr), WithClientPool()); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	cli, release := sdk.GetClient()
	if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
	release()
	if err := sdk.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	// 重复关闭是安全的
	if err := sdk.Close(); err != nil {
		t.Fatalf("重复Close error: %v", err)
	}

	// 关闭后再次获取客户端会按当前配置重建连接
	cli2, release2 := sdk.GetClient()
	defer release2()
	if _, err := cli2.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if err := sdk.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

// TestSDKInitWithConfig 验证通过WithConfig设置SDK配置
func TestSDKInitWithConfig(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	conf := &SDKConfig{Query_Addresses: []string{addr}, Query_Timeout: 1000}
	if err := sdk.Init(WithConfig(conf)); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()
	cli, release := sdk.GetClient()
	defer release()
	if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
	// WithConfig(nil)不应该panic也不应该清空已有的配置
	if err := sdk.Init(WithConfig(nil)); err != nil {
		t.Fatalf("WithConfig(nil) error: %v", err)
	}
	if len(sdk.Query_Addresses) != 1 || sdk.Query_Addresses[0] != addr {
		t.Fatalf("WithConfig(nil)不应该修改已有配置: %+v", sdk.Query_Addresses)
	}
}

// TestSDKInterceptorChain 验证多个拦截器会按顺序全部生效
func TestSDKInterceptorChain(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()

	var mu sync.Mutex
	var calls []string
	record := func(name string) grpc.UnaryClientInterceptor {
		return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			mu.Lock()
			calls = append(calls, name)
			mu.Unlock()
			return invoker(ctx, method, req, reply, cc, opts...)
		}
	}

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(WithQueryAddresses(addr), WithUnaryInterceptors(record("first"), record("second"))); err != nil {
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
	if len(calls) != 2 || calls[0] != "first" || calls[1] != "second" {
		t.Fatalf("拦截器没有按顺序全部生效: %v", calls)
	}
}

// TestSDKNewCtxMeta 验证请求元数据的设置
func TestSDKNewCtxMeta(t *testing.T) {
	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(
		WithQueryAddresses("127.0.0.1:1"),
		WithRequesterAppName("testapp"),
		WithRequesterAppVersion("1.2.3"),
	); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()

	ctx, cancel := sdk.NewCtx(sdk.WithRequestMeta(), WithMeta("custom", "value"))
	defer cancel()
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("ctx中应该包含元数据")
	}
	if got := md.Get("requester_app_name"); len(got) != 1 || got[0] != "testapp" {
		t.Fatalf("requester_app_name不正确: %v", got)
	}
	if got := md.Get("requester_app_version"); len(got) != 1 || got[0] != "1.2.3" {
		t.Fatalf("requester_app_version不正确: %v", got)
	}
	if got := md.Get("custom"); len(got) != 1 || got[0] != "value" {
		t.Fatalf("custom不正确: %v", got)
	}
}

// TestSDKGetClientConcurrent 验证并发获取客户端的正确性
func TestSDKGetClientConcurrent(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(WithQueryAddresses(addr), WithClientPool(), WithClientPoolReservations(1), WithClientPoolLimits(4), WithClientPoolAcquireWaitTime(5000)); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cli, release := sdk.GetClient()
			if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
				t.Errorf("Check error: %v", err)
			}
			release()
		}()
	}
	wg.Wait()
}

// TestSDKInitWithMultiAddresses 验证多地址时使用本地负载均衡
func TestSDKInitWithMultiAddresses(t *testing.T) {
	// 多地址带请求方名称时会开启健康检查,试验服务器需要注册同名的健康检查服务
	serviceName := "testapp-1_2_3"
	addr1, cleanup1 := startHealthServerWithServices(t, []string{serviceName})
	defer cleanup1()
	addr2, cleanup2 := startHealthServerWithServices(t, []string{serviceName})
	defer cleanup2()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(
		WithQueryAddresses(addr1, addr2),
		WithRequesterAppName("testapp"),
		WithRequesterAppVersion("1.2.3"),
	); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()
	// 多地址会构造本地负载均衡的地址
	if !strings.HasPrefix(sdk.addr, "localbalancer:///") {
		t.Fatalf("多地址时应该使用本地负载均衡的地址, got: %s", sdk.addr)
	}
	cli, release := sdk.GetClient()
	defer release()
	// 轮询多个后端都应该可以正常请求
	for i := 0; i < 4; i++ {
		if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
			t.Fatalf("Check error: %v", err)
		}
	}
}

// TestSDKInitWithDNSAddress 验证dns地址前缀时使用DNS解析与负载均衡
func TestSDKInitWithDNSAddress(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(WithQueryAddresses("dns:///" + addr)); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()
	cli, release := sdk.GetClient()
	defer release()
	if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
}

// TestSDKInitWithXDSAddress 验证xds地址前缀时使用XDS凭证初始化
func TestSDKInitWithXDSAddress(t *testing.T) {
	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(WithQueryAddresses("xds:///test-service"), WithXDSCREDS()); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()
}

// TestSDKInitAdvancedOptions 验证性能、压缩、keepalive与阻塞建连等配置项
func TestSDKInitAdvancedOptions(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(
		WithQueryAddresses(addr),
		WithMaxRecvMsgSize(4*1024*1024),
		WithMaxSendMsgSize(4*1024*1024),
		WithCompression("gzip"),
		WithKeepaliveTime(30),
		WithKeepaliveTimeout(10),
		WithKeepaliveEnforcementPermitWithoutStream(),
		WithInitialWindowSize(64*1024),
		WithInitialConnWindowSize(64*1024),
		WithConnWithBlock(),
		WithQueryTimeout(5000),
	); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()
	cli, release := sdk.GetClient()
	defer release()
	if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
}

// TestSDKGetClientPanicWhenNotInit 验证未初始化时获取客户端会panic
func TestSDKGetClientPanicWhenNotInit(t *testing.T) {
	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("未初始化时GetClient应该panic")
		}
	}()
	sdk.GetClient()
}

// TestSDKStreamInterceptor 验证流请求拦截器生效
func TestSDKStreamInterceptor(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()

	var mu sync.Mutex
	var methods []string
	streami := func(sctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		mu.Lock()
		methods = append(methods, method)
		mu.Unlock()
		return streamer(sctx, desc, cc, method, opts...)
	}

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(WithQueryAddresses(addr), WithStreamInterceptors(streami)); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()

	cli, release := sdk.GetClient()
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watchcli, err := cli.Watch(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Watch error: %v", err)
	}
	resp, err := watchcli.Recv()
	if err != nil {
		t.Fatalf("Recv error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("unexpected status: %v", resp.Status)
	}
	cancel()

	mu.Lock()
	defer mu.Unlock()
	if len(methods) != 1 || !strings.Contains(methods[0], "Watch") {
		t.Fatalf("流拦截器没有生效: %v", methods)
	}
}

// TestWithMetaOptionWithoutRequestMeta 验证单独使用WithMeta时元数据可以被正确初始化
func TestWithMetaOptionWithoutRequestMeta(t *testing.T) {
	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	ctx, cancel := sdk.NewCtx(WithMeta("key", "value"))
	defer cancel()
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("ctx中应该包含元数据")
	}
	if got := md.Get("key"); len(got) != 1 || got[0] != "value" {
		t.Fatalf("key不正确: %v", got)
	}
}

// TestWithRequestMetaWithoutConfig 验证SDKConfig为空时WithRequestMeta不会panic
func TestWithRequestMetaWithoutConfig(t *testing.T) {
	sdk := &SDK[healthpb.HealthClient]{}
	o := CtxOptions{}
	sdk.WithRequestMeta().Apply(&o)
	if o.MetaData == nil {
		t.Fatal("MetaData不应该为空")
	}
}
