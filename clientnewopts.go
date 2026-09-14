package grpcsdk

import (
	"time"

	"github.com/Golang-Tools/optparams"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DefaultBlockWaitTime 客户端使用阻塞建连(BlockUntilReady)时的默认最长等待时长
const DefaultBlockWaitTime = 10 * time.Second

// NewClientOptions 设置新建客户端的选项
type NewClientOptions struct {
	//Addr 连接地址
	Addr string
	//DialOpts 连接时的参数
	DialOpts []grpc.DialOption
	//BlockUntilReady 是否在创建客户端时阻塞等待连接就绪(等待时长由BlockWaitTime控制)
	BlockUntilReady bool
	//BlockWaitTime 阻塞等待连接就绪的最长时长,小于等于0时使用DefaultBlockWaitTime
	BlockWaitTime time.Duration
}

// DefaultNewClientOpts 默认新建客户端的选项
// 默认使用insecure的连接,和SDK未配置证书时的行为一致;
// 如果自行设置DialOpts,需要自行提供传输凭证,否则grpc会报错
var DefaultNewClientOpts = NewClientOptions{
	DialOpts: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
}

// Clone 深拷贝配置,复制DialOpts切片,避免多个客户端共享同一份配置
// @returns *NewClientOptions 拷贝出来的新配置
func (o *NewClientOptions) Clone() *NewClientOptions {
	if o == nil {
		return nil
	}
	cp := *o
	cp.DialOpts = append([]grpc.DialOption(nil), o.DialOpts...)
	return &cp
}

// WithAddr 创建客户端对象方法的参数,用于设置连接地址
// @params addr string 连接地址,如果是多个地址,则会做本地负载均衡后生成一个地址填入
func WithAddr(addr string) optparams.Option[NewClientOptions] {
	return optparams.NewFuncOption(
		func(o *NewClientOptions) {
			o.Addr = addr
		})
}

// WithDialOpts 创建客户端对象方法的参数,用于设置连接时的参数
// @params opts ...grpc.DialOption grpc的拨号设置
func WithDialOpts(opts ...grpc.DialOption) optparams.Option[NewClientOptions] {
	return optparams.NewFuncOption(
		func(o *NewClientOptions) {
			o.DialOpts = opts
		})
}

// WithClientConfig 创建客户端对象方法的参数,用于通过NewClientOptions对象设置客户端
// @params conf *NewClientOptions 设置新建客户端的选项,内部会拷贝一份,不会与外部共享
func WithClientConfig(conf *NewClientOptions) optparams.Option[NewClientOptions] {
	return optparams.NewFuncOption(
		func(o *NewClientOptions) {
			if conf == nil {
				return
			}
			*o = *conf.Clone()
		})
}

// WithBlockUntilReady 创建客户端对象方法的参数,用于设置创建客户端时阻塞等待连接就绪
func WithBlockUntilReady() optparams.Option[NewClientOptions] {
	return optparams.NewFuncOption(
		func(o *NewClientOptions) {
			o.BlockUntilReady = true
		})
}

// WithBlockWaitTime 创建客户端对象方法的参数,用于设置阻塞等待连接就绪的最长时长
// @params wait time.Duration 最长等待时长,小于等于0时使用DefaultBlockWaitTime
func WithBlockWaitTime(wait time.Duration) optparams.Option[NewClientOptions] {
	return optparams.NewFuncOption(
		func(o *NewClientOptions) {
			o.BlockWaitTime = wait
		})
}
