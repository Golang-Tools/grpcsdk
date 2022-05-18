package grpcsdk

import (
	"context"
	"sync"

	"github.com/Golang-Tools/optparams"
	"google.golang.org/grpc/connectivity"
)

//GrpcConnPool 客户端连接池,满足接口GrpcClientGetter
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
type GrpcConnPool[T any] struct {
	opts          NewClientPoolOptions
	clis          chan GrpcClientInterface[T]
	mu            *sync.RWMutex
	clientfactory NewGrpcClientFunc[T]
}

//fillConns 向conns中填入指定数量的连接
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@params n int 填入的数量
//@returns error 错误信息
func (p *GrpcConnPool[T]) fillConns(n int) error {
	for i := 0; i < n; i++ {
		cli, err := NewClient(p.clientfactory, WithClientConfig(p.opts.NewClientOptions))
		if err != nil {
			return err
		}

		p.clis <- cli
	}
	return nil
}

//acquire 从池中获取连接
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@returns GrpcClientInterface[T] 客户端接口对象
//@returns error 错误信息
func (p *GrpcConnPool[T]) acquire() (GrpcClientInterface[T], error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.opts.AcquireWaitTime)
	defer cancel()
	var cli GrpcClientInterface[T]
	for {
		select {
		case cli = <-p.clis:
			conn := cli.GetConn()
			switch conn.GetState() {
			case connectivity.Idle, connectivity.Ready:
				{
					return cli, nil
				}
			case connectivity.Connecting:
				{
					if len(p.clis) < p.opts.Reservations {
						p.clis <- cli
					} else {
						cli.Close()
					}
				}
			case connectivity.TransientFailure:
				{
					if len(p.clis) < p.opts.Reservations {
						cli.GetConn().Connect()
						p.clis <- cli
					} else {
						cli.Close()
					}
				}
			}
		case <-ctx.Done():
			return nil, ErrTimeout // it would better returns ctx.Err()
		}
	}
}

//Acquire 获取客户端接口
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@params opts ...optparams.Option[AcquireOptions] acquire方法的参数,只有`Force()`可用,表示是否强制获取,如果队列中已经没有可用的客户端了则创建一个客户端对象
//@returns GrpcClientInterface[T] 客户端接口对象
//@returns error 错误信息
func (p *GrpcConnPool[T]) Acquire(opts ...optparams.Option[AcquireOptions]) (GrpcClientInterface[T], error) {
	c, err := p.acquire()
	if err != nil {
		dopt := AcquireOptions{}
		optparams.GetOption(&dopt, opts...)
		if dopt.Force {
			if err != ErrTimeout {
				return nil, err
			}
			cli, err := NewClient(p.clientfactory, WithClientConfig(p.opts.NewClientOptions))
			if err != nil {
				return nil, err
			}
			return cli, nil
		}
		return nil, err
	}
	return c, nil
}

//Release 释放连接放回池中
//关闭状态的连接不会被放入池中
//当池子当前容量小于设置的最小值时才会放回,否则关闭连接
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@params cli GrpcClientInterface[T] 要释放的客户端资源
func (p *GrpcConnPool[T]) Release(cli GrpcClientInterface[T]) {
	p.mu.Lock()
	defer p.mu.Unlock()
	conn := cli.GetConn()
	switch conn.GetState() {
	case connectivity.Idle, connectivity.Connecting, connectivity.Ready, connectivity.TransientFailure:
		{
			if len(p.clis) < p.opts.Reservations {
				p.clis <- cli
			} else {
				cli.Close()
			}
		}
	}
}

//IsClosed 监测池是否已经关闭
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@returns bool 是否已经关闭
func (p *GrpcConnPool[T]) IsClosed() bool {
	if p == nil {
		return true
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.clis == nil
}

//Limits 返回池最大容量限制
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@returns int 最大容量
func (p *GrpcConnPool[T]) Limits() int {
	return p.opts.Limits
}

//Available 获取当前池可用的客户端数
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@returns int 可用客户端数
func (p *GrpcConnPool[T]) Available() int {
	if p.IsClosed() {
		return 0
	}
	return len(p.clis)
}

//Close 关闭连接池
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@returns error 错误信息
func (p *GrpcConnPool[T]) Close() error {
	p.mu.Lock()
	clis := p.clis
	p.clis = nil
	p.mu.Unlock()

	if clis == nil {
		return nil
	}
	close(clis)
	var err error
	for cli := range clis {
		err = cli.GetConn().Close()
	}
	return err
}

//NewPool 创建一个池
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@params newclientfunc NewGrpcClientFunc[T] 将grpc连接转化为grpc客户端的程序,可以在pb生成的模块中找到,通常以`NewXXXXXXClient`命名
//@params opts ...optparams.Option[NewClientPoolOptions] 创建池对象的参数选项,详细可以看clientpoolnewopts.go文件
//@returns *GrpcConnPool[T] 返回客户端连接池对象
//@returns error 错误信息
func NewPool[T any](newgrpcclientfunc NewGrpcClientFunc[T], opts ...optparams.Option[NewClientPoolOptions]) (*GrpcConnPool[T], error) {
	p := new(GrpcConnPool[T])
	p.clientfactory = newgrpcclientfunc
	p.mu = &sync.RWMutex{}
	p.opts = DefaultNewClientPoolOpts
	optparams.GetOption(&p.opts, opts...)
	for _, opt := range opts {
		opt.Apply(&p.opts)
	}
	if p.opts.Reservations < 1 {
		return nil, ErrReservationSmallThanOne
	}
	if p.opts.Limits < p.opts.Reservations {
		return nil, ErrLimitsSmallThanReservation
	}

	p.clis = make(chan GrpcClientInterface[T], p.opts.Limits)
	p.mu.Lock()
	defer p.mu.Unlock()
	err := p.fillConns(p.opts.Reservations)
	if err != nil {
		return nil, err
	}
	return p, nil
}
