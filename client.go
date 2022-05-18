package grpcsdk

import (
	"github.com/Golang-Tools/optparams"
	grpc "google.golang.org/grpc"
)

//Client 客户端类,满足接口GrpcClientGetter和GrpcClientInterface
//@generics T any 需要指定返回的客户端接口
type Client[T any] struct {
	cli           T
	conn          *grpc.ClientConn
	opts          NewClientOptions
	clientfactory NewGrpcClientFunc[T]
}

//Acquire  维持接口,返回自身
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@params opts ...optparams.Option[AcquireOptions] acquire方法的参数,只有`Force()`可用,不会生效
//@returns GrpcClientInterface[T] 客户端接口对象
//@returns error 错误信息
func (p *Client[T]) Acquire(opts ...optparams.Option[AcquireOptions]) (GrpcClientInterface[T], error) {
	return p, nil
}

//Release 释放客户端资源,维持接口不做任何操作
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@params cli GrpcClientInterface[T] 要释放的客户端资源
func (p *Client[T]) Release(cli GrpcClientInterface[T]) {
}

//GetConn 获取客户端资源,即获取自身
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@returns GrpcConnInterface 返回自身
func (c *Client[T]) GetConn() GrpcConnInterface {
	return c.conn
}

//AsGrpcClient 将对象转成满足T接口的对象,此处返回自身
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@returns T 返回自身
func (c *Client[T]) AsGrpcClient() T {
	return c.cli
}

//NewClient 创建一个客户端对象
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@params newgrpcclientfunc NewGrpcClientFunc[T] 将grpc连接转化为grpc客户端的程序,可以在pb生成的模块中找到,通常以`NewXXXXXXClient`命名
//@params opts ...optparams.Option[NewClientOptions] 创建客户端的配置项,详细可以看clientnewopts.go文件
//@returns *Client[T] 返回客户端对象
//@returns error 错误信息
func NewClient[T any](newgrpcclientfunc NewGrpcClientFunc[T], opts ...optparams.Option[NewClientOptions]) (*Client[T], error) {
	c := new(Client[T])
	c.clientfactory = newgrpcclientfunc
	c.opts = DefaultNewClientOpts
	optparams.GetOption(&c.opts, opts...)
	conn, err := grpc.Dial(c.opts.Addr, c.opts.DialOpts...)
	if err != nil {
		return nil, err
	}
	c.conn = conn
	c.cli = newgrpcclientfunc(conn)
	return c, nil
}

//Close 断开连接
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@returns error 错误信息
func (c *Client[T]) Close() error {
	return c.GetConn().Close()
}
