package grpcsdk

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/connectivity"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// TestNewPool 验证池的创建、获取、释放与关闭
func TestNewPool(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()
	pool, err := NewPool(healthFactory(), HasClientConfig(WithAddr(addr)), WithReservations(2), WithLimits(4))
	if err != nil {
		t.Fatalf("NewPool error: %v", err)
	}
	if pool.Limits() != 4 {
		t.Fatalf("池最大容量应该为4, got: %v", pool.Limits())
	}
	if pool.Available() != 2 {
		t.Fatalf("池注水后应该有2个可用的客户端, got: %v", pool.Available())
	}
	if pool.IsClosed() {
		t.Fatal("新建的池不应该处于关闭状态")
	}

	cli, err := pool.Acquire()
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	if _, err := cli.AsGrpcClient().Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
	pool.Release(cli)
	if pool.Available() != 2 {
		t.Fatalf("释放连接后池中应该回到安全水位, got: %v", pool.Available())
	}

	if err := pool.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if !pool.IsClosed() {
		t.Fatal("关闭后池应该处于关闭状态")
	}
	if pool.Available() != 0 {
		t.Fatalf("关闭后池中不应该有可用客户端, got: %v", pool.Available())
	}
	// 重复关闭是安全的
	if err := pool.Close(); err != nil {
		t.Fatalf("重复Close error: %v", err)
	}
	// 关闭后获取应该返回ErrClosed
	if _, err := pool.Acquire(); !errors.Is(err, ErrClosed) {
		t.Fatalf("关闭后获取应该返回ErrClosed, got: %v", err)
	}
	// 关闭后释放不应该阻塞
	done := make(chan struct{})
	go func() {
		pool.Release(cli)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("关闭池后调用Release阻塞了")
	}
}

// TestNewPoolFromNilClient 验证nil的池对象的方法调用
func TestNilPool(t *testing.T) {
	var pool *GrpcConnPool[healthpb.HealthClient]
	if !pool.IsClosed() {
		t.Fatal("nil的池应该被认为已经关闭")
	}
	if got := pool.Available(); got != 0 {
		t.Fatalf("nil的池可用客户端数应该为0, got: %v", got)
	}
	if got := pool.Limits(); got != 0 {
		t.Fatalf("nil的池最大容量应该为0, got: %v", got)
	}
}

// TestPoolAcquireTimeoutAndForce 验证池中没有可用客户端时的超时与Force行为
func TestPoolAcquireTimeoutAndForce(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()
	pool, err := NewPool(healthFactory(), HasClientConfig(WithAddr(addr)), WithReservations(1), WithLimits(1), WithAcquireWaitTimeMS(100))
	if err != nil {
		t.Fatalf("NewPool error: %v", err)
	}
	defer pool.Close()

	cli, err := pool.Acquire()
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	// 池中已经没有可用的客户端了,获取会超时
	if _, err := pool.Acquire(); !errors.Is(err, ErrTimeout) {
		t.Fatalf("池空时获取应该超时, got: %v", err)
	}
	// Force时会新建一个客户端对象
	forcecli, err := pool.Acquire(Force())
	if err != nil {
		t.Fatalf("Force Acquire error: %v", err)
	}
	if _, err := forcecli.AsGrpcClient().Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
	pool.Release(forcecli)
	pool.Release(cli)
}

// TestPoolConcurrent 验证池在并发获取/释放下的正确性
func TestPoolConcurrent(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()
	pool, err := NewPool(healthFactory(), HasClientConfig(WithAddr(addr)), WithReservations(2), WithLimits(8), WithAcquireWaitTimeMS(5000))
	if err != nil {
		t.Fatalf("NewPool error: %v", err)
	}
	defer pool.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				cli, err := pool.Acquire()
				if err != nil {
					t.Errorf("Acquire error: %v", err)
					return
				}
				if _, err := cli.AsGrpcClient().Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
					t.Errorf("Check error: %v", err)
				}
				pool.Release(cli)
			}
		}()
	}
	wg.Wait()
	// 并发结束后池不应该处于异常状态
	if pool.IsClosed() {
		t.Fatal("并发结束后池不应该处于关闭状态")
	}
}

// TestPoolCloseConcurrent 验证池在并发获取/关闭时的正确性
func TestPoolCloseConcurrent(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()
	pool, err := NewPool(healthFactory(), HasClientConfig(WithAddr(addr)), WithReservations(1), WithLimits(4), WithAcquireWaitTimeMS(100))
	if err != nil {
		t.Fatalf("NewPool error: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				cli, err := pool.Acquire()
				if err != nil {
					// 池被关闭后获取会失败,或者获取超时,都是可以接受的结果
					if !errors.Is(err, ErrClosed) && !errors.Is(err, ErrTimeout) {
						t.Errorf("unexpected Acquire error: %v", err)
					}
					return
				}
				pool.Release(cli)
			}
		}()
	}
	time.Sleep(10 * time.Millisecond)
	if err := pool.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	wg.Wait()
	if !pool.IsClosed() {
		t.Fatal("池应该处于关闭状态")
	}
}

// TestPoolOptionsIsolation 验证多个池的配置互不影响
func TestPoolOptionsIsolation(t *testing.T) {
	addr1, cleanup1 := startHealthServer(t)
	defer cleanup1()
	addr2, cleanup2 := startHealthServer(t)
	defer cleanup2()
	factory := healthFactory()

	pool1, err := NewPool(factory, HasClientConfig(WithAddr(addr1)), WithReservations(1), WithLimits(1))
	if err != nil {
		t.Fatalf("NewPool error: %v", err)
	}
	defer pool1.Close()
	pool2, err := NewPool(factory, HasClientConfig(WithAddr(addr2)), WithReservations(1), WithLimits(1))
	if err != nil {
		t.Fatalf("NewPool error: %v", err)
	}
	defer pool2.Close()

	if pool1.opts.NewClientOptions.Addr != addr1 || pool2.opts.NewClientOptions.Addr != addr2 {
		t.Fatalf("池的配置互相污染: %v, %v", pool1.opts.NewClientOptions.Addr, pool2.opts.NewClientOptions.Addr)
	}

	// 两个池都可以正常获取并使用客户端
	cli1, err := pool1.Acquire()
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	defer pool1.Release(cli1)
	cli2, err := pool2.Acquire()
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	defer pool2.Release(cli2)
	if _, err := cli1.AsGrpcClient().Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if _, err := cli2.AsGrpcClient().Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
}

// TestPoolReleaseBeyondReservations 验证超过安全水位的连接释放时会被关闭
func TestPoolReleaseBeyondReservations(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()
	pool, err := NewPool(healthFactory(), HasClientConfig(WithAddr(addr)), WithReservations(1), WithLimits(3), WithAcquireWaitTimeMS(100))
	if err != nil {
		t.Fatalf("NewPool error: %v", err)
	}
	defer pool.Close()

	c1, err := pool.Acquire()
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	// 使用Force获取一个池外新建的客户端
	c2, err := pool.Acquire(Force())
	if err != nil {
		t.Fatalf("Force Acquire error: %v", err)
	}
	// 池中可用客户端数小于安全水位时,放回的连接会被保留
	pool.Release(c1)
	if got := pool.Available(); got != 1 {
		t.Fatalf("期望池中可用客户端数为1, got: %v", got)
	}
	// 池中可用客户端数达到安全水位后,继续放回的连接会被关闭
	pool.Release(c2)
	if got := pool.Available(); got != 1 {
		t.Fatalf("超过安全水位后放回的连接应该被关闭, got: %v", got)
	}
	// 池中仍然可以获取到可用的客户端
	c3, err := pool.Acquire()
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	if _, err := c3.AsGrpcClient().Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
	pool.Release(c3)
}

// TestPoolFillConnsPartialFailure 验证注水中途失败时会清理已经创建好的连接
func TestPoolFillConnsPartialFailure(t *testing.T) {
	addr, cleanup := startDirtyHealthServer(t)
	defer cleanup()

	// 第一个连接可以正常建立,之后的连接会被服务器立即断开
	pool, err := NewPool(
		healthFactory(),
		HasClientConfig(WithAddr(addr), WithBlockUntilReady(), WithBlockWaitTime(300*time.Millisecond)),
		WithReservations(2),
		WithLimits(2),
	)
	if err == nil {
		pool.Close()
		t.Fatal("注水中途失败时NewPool应该返回错误")
	}
}

// TestPoolAcquireTransientFailure 验证连接故障时获取客户端会重试直到超时
func TestPoolAcquireTransientFailure(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	pool, err := NewPool(healthFactory(), HasClientConfig(WithAddr(addr)), WithReservations(1), WithLimits(1), WithAcquireWaitTimeMS(200))
	if err != nil {
		t.Fatalf("NewPool error: %v", err)
	}
	defer pool.Close()

	cli, err := pool.Acquire()
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	// 关闭服务器,让连接进入故障状态
	cleanup()
	if _, err := cli.AsGrpcClient().Check(context.Background(), &healthpb.HealthCheckRequest{}); err == nil {
		t.Fatal("服务器关闭后请求应该报错")
	}
	// 等待连接进入TransientFailure状态
	deadline := time.Now().Add(2 * time.Second)
	for cli.GetConn().GetState() != connectivity.TransientFailure {
		if time.Now().After(deadline) {
			t.Fatalf("连接没有进入TransientFailure状态, got: %v", cli.GetConn().GetState())
		}
		time.Sleep(10 * time.Millisecond)
	}
	// 故障状态的连接会被按安全水位放回池中
	pool.Release(cli)
	// 连接持续故障时获取会重试直到超时
	if _, err := pool.Acquire(); !errors.Is(err, ErrTimeout) {
		t.Fatalf("连接故障时获取应该超时, got: %v", err)
	}
}

// TestPoolReleaseShutdownConn 验证释放已经关闭的连接时不会放回池中
func TestPoolReleaseShutdownConn(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()
	pool, err := NewPool(healthFactory(), HasClientConfig(WithAddr(addr)), WithReservations(1), WithLimits(1), WithAcquireWaitTimeMS(50))
	if err != nil {
		t.Fatalf("NewPool error: %v", err)
	}
	defer pool.Close()

	cli, err := pool.Acquire()
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	// 手动关闭连接,模拟连接已经进入Shutdown状态
	if err := cli.GetConn().Close(); err != nil {
		t.Fatalf("Close conn error: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for cli.GetConn().GetState() != connectivity.Shutdown {
		if time.Now().After(deadline) {
			t.Fatalf("连接没有进入Shutdown状态, got: %v", cli.GetConn().GetState())
		}
		time.Sleep(5 * time.Millisecond)
	}
	pool.Release(cli)
	// 已关闭的连接不会被放回池中
	if got := pool.Available(); got != 0 {
		t.Fatalf("已经关闭的连接不应该被放回池中, got: %v", got)
	}
}

// TestPoolTryPutbackWhenChannelFull 验证池已满时放回连接会失败并关闭连接
// 正常配置下池的容量不会小于安全水位,这里直接构造异常状态验证防御分支
func TestPoolTryPutbackWhenChannelFull(t *testing.T) {
	addr, cleanup := startHealthServer(t)
	defer cleanup()
	factory := healthFactory()

	pool := &GrpcConnPool[healthpb.HealthClient]{
		opts: NewClientPoolOptions{
			NewClientOptions: &NewClientOptions{Addr: addr, DialOpts: DefaultNewClientOpts.Clone().DialOpts},
			Reservations:     5,
			Limits:           1,
			AcquireWaitTime:  time.Millisecond,
		},
		clis:          make(chan GrpcClientInterface[healthpb.HealthClient], 1),
		mu:            &sync.RWMutex{},
		done:          make(chan struct{}),
		clientfactory: factory,
	}

	c1, err := NewClient(factory, WithAddr(addr))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	if !pool.tryPutback(c1) {
		t.Fatal("低于安全水位时应该可以放回池中")
	}
	c2, err := NewClient(factory, WithAddr(addr))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	if pool.tryPutback(c2) {
		t.Fatal("池已经放满时不应该可以放回")
	}
	// 放回失败的连接会被关闭
	if got := c2.GetConn().GetState(); got != connectivity.Shutdown {
		t.Fatalf("放回失败的连接应该被关闭, got: %v", got)
	}
	if err := pool.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}
