package grpcsdk

import (
	"github.com/Golang-Tools/optparams"
	"google.golang.org/grpc"
)

//NewClientOptions 设置新建客户端的选项
type NewClientOptions struct {
	Addr     string
	DialOpts []grpc.DialOption
}

//DefaultNewClientOpts 默认新建客户端的选项
var DefaultNewClientOpts = NewClientOptions{
	DialOpts: []grpc.DialOption{},
}

//WithAddr 创建客户端对象方法的参数,用于设置连接地址
//@params addr string 连接地址,如果是多个地址,则会做本地负载均衡后生成一个地址填入
func WithAddr(addr string) optparams.Option[NewClientOptions] {
	return optparams.NewFuncOption(
		func(o *NewClientOptions) {
			o.Addr = addr
		})
}

//WithDialOpts 创建客户端对象方法的参数,用于设置连接时的参数
//@params opts ...grpc.DialOption grpc的拨号设置
func WithDialOpts(opts ...grpc.DialOption) optparams.Option[NewClientOptions] {
	return optparams.NewFuncOption(
		func(o *NewClientOptions) {
			o.DialOpts = opts
		})
}

//WithClientConfig 创建客户端对象方法的参数,用于通过NewClientOptions对象设置客户端
//@params conf *NewClientOptions 设置新建客户端的选项
func WithClientConfig(conf *NewClientOptions) optparams.Option[NewClientOptions] {
	return optparams.NewFuncOption(
		func(o *NewClientOptions) {
			o.Addr = conf.Addr
			o.DialOpts = conf.DialOpts
		})
}
