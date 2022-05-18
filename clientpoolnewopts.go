package grpcsdk

import (
	"time"

	"github.com/Golang-Tools/optparams"
	"google.golang.org/grpc"
)

//NewClientPoolOptions 设置新建客户端池的选项
type NewClientPoolOptions struct {
	*NewClientOptions
	Reservations    int
	Limits          int
	AcquireWaitTime time.Duration
}

//DefaultNewClientPoolOpts 默认新建客户端池的选项
var DefaultNewClientPoolOpts = NewClientPoolOptions{
	NewClientOptions: &NewClientOptions{DialOpts: []grpc.DialOption{}},
	Reservations:     1,
	Limits:           3,
	AcquireWaitTime:  time.Duration(1) * time.Millisecond,
}

//HasClientConfig 复用创建客户端的选项用于创建池
//@params opts ...optparams.Option[NewClientOptions] 用于创建客户端的配置项
func HasClientConfig(opts ...optparams.Option[NewClientOptions]) optparams.Option[NewClientPoolOptions] {
	return optparams.NewFuncOption(
		func(o *NewClientPoolOptions) {
			optparams.GetOption(o.NewClientOptions, opts...)
		})
}

//WithReservations 创建池对象方法的参数,用于设置连接池的最小连接数
//@params reservations int 池安全水位
func WithReservations(reservations int) optparams.Option[NewClientPoolOptions] {

	return optparams.NewFuncOption(
		func(o *NewClientPoolOptions) {
			o.Reservations = reservations
		})
}

//WithLimits 创建池对象方法的参数,用于设置连接池的最大连接数
//@params limits int 池最大水位
func WithLimits(limits int) optparams.Option[NewClientPoolOptions] {

	return optparams.NewFuncOption(
		func(o *NewClientPoolOptions) {
			o.Limits = limits
		})
}

//WithAcquireWaitTimeMS 创建池对象方法的参数,用于设置从连接池中获取连接的最大等待时长,单位ms.
//@params wait int 获取客户端等待时长
func WithAcquireWaitTimeMS(wait int) optparams.Option[NewClientPoolOptions] {
	return optparams.NewFuncOption(
		func(o *NewClientPoolOptions) {
			o.AcquireWaitTime = time.Duration(wait) * time.Millisecond
		})
}
