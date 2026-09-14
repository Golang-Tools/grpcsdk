package grpcsdk

import (
	"context"
	"errors"
	"testing"
	"time"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// TestNewClient 验证客户端的创建、调用、获取接口与关闭
func TestNewClient(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()
	cli, err := NewClient(healthFactory(), WithAddr(addr))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	defer cli.Close()

	resp, err := cli.AsGrpcClient().Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("unexpected status: %v", resp.Status)
	}

	if cli.GetConn() == nil {
		t.Fatal("GetConn不应该返回nil")
	}

	// Acquire/Release 维持接口,返回自身
	got, err := cli.Acquire()
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	if got == nil || got.AsGrpcClient() == nil {
		t.Fatal("Acquire不应该返回nil")
	}
	cli.Release(got)
}

// TestNewClientValidation 验证NewClient的参数校验
func TestNewClientValidation(t *testing.T) {
	if _, err := NewClient[healthpb.HealthClient](nil); !errors.Is(err, ErrNilClientFactory) {
		t.Fatalf("factory为nil时应该返回ErrNilClientFactory, got: %v", err)
	}
	if _, err := NewClient(healthFactory()); !errors.Is(err, ErrEmptyAddr) {
		t.Fatalf("地址为空时应该返回ErrEmptyAddr, got: %v", err)
	}
}

// TestNewClientBlockUntilReady 验证阻塞建连
func TestNewClientBlockUntilReady(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()
	cli, err := NewClient(healthFactory(), WithAddr(addr), WithBlockUntilReady(), WithBlockWaitTime(time.Second))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	defer cli.Close()
	if state := cli.GetConn().GetState(); state != connectivity.Ready {
		t.Fatalf("阻塞建连后连接应该处于Ready状态, got: %v", state)
	}
}

// TestNewClientBlockUntilReadyTimeout 验证阻塞建连在连接不可达时超时
func TestNewClientBlockUntilReadyTimeout(t *testing.T) {
	addr := unusedAddr(t)
	_, err := NewClient(healthFactory(), WithAddr(addr), WithBlockUntilReady(), WithBlockWaitTime(200*time.Millisecond))
	if err == nil {
		t.Fatal("连接不可达时阻塞建连应该超时")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("期待超时错误, got: %v", err)
	}
}

// TestNewClientOptionsIsolation 验证多个客户端的配置互不影响
func TestNewClientOptionsIsolation(t *testing.T) {
	addr1, cleanup1 := startHealthServer(t)
	defer cleanup1()
	addr2, cleanup2 := startHealthServer(t)
	defer cleanup2()
	factory := healthFactory()

	cli1, err := NewClient(factory, WithAddr(addr1))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	defer cli1.Close()
	cli2, err := NewClient(factory, WithAddr(addr2))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	defer cli2.Close()

	if cli1.opts.Addr != addr1 || cli2.opts.Addr != addr2 {
		t.Fatalf("客户端的配置互相污染: %v, %v", cli1.opts.Addr, cli2.opts.Addr)
	}
}

// TestWaitConnReadyOnClosedConn 验证等待已经关闭的连接时会立即返回错误
func TestWaitConnReadyOnClosedConn(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	_ = conn.Close()
	if err := waitConnReady(conn, time.Second); !errors.Is(err, ErrAlreadyClosed) {
		t.Fatalf("连接已关闭时应该返回ErrAlreadyClosed, got: %v", err)
	}
}

// TestClientCloseWithoutConn 验证没有连接的客户端对象关闭时不会panic
func TestClientCloseWithoutConn(t *testing.T) {
	c := &Client[healthpb.HealthClient]{}
	if err := c.Close(); err != nil {
		t.Fatalf("没有连接时Close不应该报错, got: %v", err)
	}
}

// TestOptionsNilHelpers 验证配置辅助方法对nil输入的处理
func TestOptionsNilHelpers(t *testing.T) {
	var o *NewClientOptions
	if o.Clone() != nil {
		t.Fatal("nil配置的Clone应该返回nil")
	}
	var po *NewClientPoolOptions
	if got := po.Clone(); got.Reservations != 0 {
		t.Fatalf("nil池配置的Clone应该返回零值配置, got: %+v", got)
	}
	base := NewClientOptions{Addr: "example.com:1234"}
	WithClientConfig(nil).Apply(&base)
	if base.Addr != "example.com:1234" {
		t.Fatalf("WithClientConfig(nil)不应该修改配置, got: %+v", base)
	}
}
