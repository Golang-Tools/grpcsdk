package grpcsdk

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	log "github.com/Golang-Tools/loggerhelper/v4"
	"github.com/Golang-Tools/optparams"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// TestMain 测试入口,测试期间把日志输出到io.Discard避免干扰测试输出
func TestMain(m *testing.M) {
	log.Set(log.WithOutput(io.Discard))
	os.Exit(m.Run())
}

// startHealthServer 启动一个带健康检查服务的grpc测试服务器
// @params t *testing.T 测试对象
// @returns string 服务地址
// @returns func() 关闭服务器的清理函数
func startHealthServer(t *testing.T) (string, func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test server error: %v", err)
	}
	s := grpc.NewServer()
	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(s, hs)
	go func() {
		_ = s.Serve(lis)
	}()
	return lis.Addr().String(), func() {
		s.Stop()
		_ = lis.Close()
	}
}

// unusedAddr 返回一个没有被监听的本地地址
// @params t *testing.T 测试对象
// @returns string 地址
func unusedAddr(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	addr := lis.Addr().String()
	_ = lis.Close()
	return addr
}

// healthFactory 返回测试用客户端构造函数
// @returns NewGrpcClientFunc[healthpb.HealthClient] 构造函数
func healthFactory() NewGrpcClientFunc[healthpb.HealthClient] {
	return func(conn grpc.ClientConnInterface) healthpb.HealthClient {
		return healthpb.NewHealthClient(conn)
	}
}

// TestNewPoolValidation 验证NewPool的参数校验
func TestNewPoolValidation(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()
	factory := healthFactory()
	cases := []struct {
		name    string
		factory NewGrpcClientFunc[healthpb.HealthClient]
		opts    []optparams.Option[NewClientPoolOptions]
		expect  error
	}{
		{
			name:    "工厂为nil",
			factory: nil,
			opts:    []optparams.Option[NewClientPoolOptions]{HasClientConfig(WithAddr(addr))},
			expect:  ErrNilClientFactory,
		},
		{
			name:    "地址为空",
			factory: factory,
			expect:  ErrEmptyAddr,
		},
		{
			name:    "reservations小于1",
			factory: factory,
			opts:    []optparams.Option[NewClientPoolOptions]{HasClientConfig(WithAddr(addr)), WithReservations(0)},
			expect:  ErrReservationSmallThanOne,
		},
		{
			name:    "limits小于reservations",
			factory: factory,
			opts:    []optparams.Option[NewClientPoolOptions]{HasClientConfig(WithAddr(addr)), WithReservations(3), WithLimits(2)},
			expect:  ErrLimitsSmallThanReservation,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewPool(c.factory, c.opts...)
			if !errors.Is(err, c.expect) {
				t.Fatalf("期待错误%v,实际为%v", c.expect, err)
			}
		})
	}
}

// TestDefaultOptionsNotPolluted 验证创建客户端与池不会污染包级的默认配置
func TestDefaultOptionsNotPolluted(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()
	factory := healthFactory()

	cli, err := NewClient(factory, WithAddr(addr))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	defer cli.Close()
	if DefaultNewClientOpts.Addr != "" {
		t.Fatalf("默认客户端配置被污染: %+v", DefaultNewClientOpts)
	}
	if cli.opts.Addr != addr {
		t.Fatalf("客户端的配置不正确: %+v", cli.opts)
	}

	pool, err := NewPool(factory, HasClientConfig(WithAddr(addr)), WithReservations(1), WithLimits(1))
	if err != nil {
		t.Fatalf("NewPool error: %v", err)
	}
	defer pool.Close()
	if DefaultNewClientPoolOpts.NewClientOptions.Addr != "" {
		t.Fatalf("默认池配置被污染: %+v", DefaultNewClientPoolOpts.NewClientOptions)
	}
	if pool.opts.NewClientOptions == nil || pool.opts.NewClientOptions.Addr != addr {
		t.Fatalf("池的配置不正确: %+v", pool.opts.NewClientOptions)
	}

	// HasClientConfig不应该修改共享的默认配置
	before := *DefaultNewClientPoolOpts.NewClientOptions
	popt := NewClientPoolOptions{}
	HasClientConfig(WithAddr("example.com:1234")).Apply(&popt)
	if popt.NewClientOptions == nil || popt.NewClientOptions.Addr != "example.com:1234" {
		t.Fatalf("HasClientConfig没有正确设置配置: %+v", popt.NewClientOptions)
	}
	if DefaultNewClientPoolOpts.NewClientOptions.Addr != before.Addr {
		t.Fatalf("HasClientConfig污染了默认配置: %+v", DefaultNewClientPoolOpts.NewClientOptions)
	}
}

// TestSDKLoggerNotPolluteGlobalExtFields 验证创建SDK不会破坏全局日志的扩展字段
func TestSDKLoggerNotPolluteGlobalExtFields(t *testing.T) {
	var buf bytes.Buffer
	log.Set(log.WithOutput(&buf), log.WithExtFields(map[string]interface{}{"app": "grpcsdk-test"}))
	defer log.Set(log.WithOutput(io.Discard), log.WithExtFields(map[string]interface{}{}))

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if sdk.Logger == nil {
		t.Fatal("SDK的Logger不应该为空")
	}

	buf.Reset()
	log.Info("check global ext fields")
	if !strings.Contains(buf.String(), "grpcsdk-test") {
		t.Fatalf("创建SDK后全局扩展字段被清空, got: %s", buf.String())
	}

	// SDK自带的Logger应该带有module与target_service字段
	buf.Reset()
	sdk.Logger.Info("sdk logger")
	out := buf.String()
	if !strings.Contains(out, "grpcsdk") || !strings.Contains(out, "target_service") {
		t.Fatalf("SDK的Logger缺少module/target_service字段, got: %s", out)
	}
}

// TestSDKNewWithoutDesc 验证没有服务描述对象时创建SDK不会panic
func TestSDKNewWithoutDesc(t *testing.T) {
	sdk := New(healthFactory(), nil)
	if sdk.Logger == nil {
		t.Fatal("SDK的Logger不应该为空")
	}
}

// TestSDKInitNoAddress 验证没有配置地址时Init的错误
func TestSDKInitNoAddress(t *testing.T) {
	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(); !errors.Is(err, ErrNoAddresses) {
		t.Fatalf("没有地址时应该返回ErrNoAddresses, got: %v", err)
	}
}

// TestSDKInitTLSFileError 验证CA证书文件读取失败时的错误
func TestSDKInitTLSFileError(t *testing.T) {
	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	err := sdk.Init(WithQueryAddresses("127.0.0.1:1"), WithCaCertPath("/not/exist/ca.pem"))
	if err == nil {
		t.Fatal("CA证书不存在时Init应该报错")
	}
}

// TestSDKNewCtx 验证NewCtx的超时行为
func TestSDKNewCtx(t *testing.T) {
	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)

	// 默认没有配置超时时间时ctx不会超时
	ctx, cancel := sdk.NewCtx()
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("默认情况下ctx不应该有超时时间")
	}
	cancel()

	// 显式设置超时时间,即使SDK没有配置Query_Timeout也应该生效
	ctx, cancel = sdk.NewCtx(WithTimeout(50 * time.Millisecond))
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("显式设置WithTimeout后ctx应该有超时时间")
	}
	cancel()

	// SDK配置了Query_Timeout时使用配置的超时时间
	sdk.Query_Timeout = 100
	ctx, cancel = sdk.NewCtx()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("配置Query_Timeout后ctx应该有超时时间")
	}
	cancel()

	// UntilEnd的优先级最高
	ctx, cancel = sdk.NewCtx(UntilEnd(), WithTimeout(50*time.Millisecond))
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("UntilEnd时ctx不应该有超时时间")
	}
	cancel()
}
