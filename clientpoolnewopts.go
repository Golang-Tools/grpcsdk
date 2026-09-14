package grpcsdk

import (
	"time"

	"github.com/Golang-Tools/optparams"
)

// NewClientPoolOptions 设置新建客户端池的选项
type NewClientPoolOptions struct {
	*NewClientOptions
	//Reservations 池安全水位,池中保持的客户端数量不低于该值
	Reservations int
	//Limits 池最大水位
	Limits int
	//AcquireWaitTime 从池中获取客户端的最大等待时长
	AcquireWaitTime time.Duration
}

// DefaultNewClientPoolOpts 默认新建客户端池的选项
var DefaultNewClientPoolOpts = NewClientPoolOptions{
	NewClientOptions: DefaultNewClientOpts.Clone(),
	Reservations:     1,
	Limits:           3,
	AcquireWaitTime:  time.Duration(1) * time.Millisecond,
}

// Clone 深拷贝配置(包含内嵌的NewClientOptions),避免多个池共享同一份配置
// @returns NewClientPoolOptions 拷贝出来的新配置
func (o *NewClientPoolOptions) Clone() NewClientPoolOptions {
	if o == nil {
		return NewClientPoolOptions{}
	}
	cp := *o
	cp.NewClientOptions = o.NewClientOptions.Clone()
	return cp
}

// HasClientConfig 复用创建客户端的选项用于创建池
// @params opts ...optparams.Option[NewClientOptions] 用于创建客户端的配置项
func HasClientConfig(opts ...optparams.Option[NewClientOptions]) optparams.Option[NewClientPoolOptions] {
	return optparams.NewFuncOption(
		func(o *NewClientPoolOptions) {
			base := o.NewClientOptions
			if base == nil {
				base = &NewClientOptions{}
			}
			// 基于当前配置的拷贝解析,避免修改共享的默认配置
			o.NewClientOptions = optparams.GetOption(base, opts...)
		})
}

// WithReservations 创建池对象方法的参数,用于设置连接池的最小连接数
// @params reservations int 池安全水位
func WithReservations(reservations int) optparams.Option[NewClientPoolOptions] {

	return optparams.NewFuncOption(
		func(o *NewClientPoolOptions) {
			o.Reservations = reservations
		})
}

// WithLimits 创建池对象方法的参数,用于设置连接池的最大连接数
// @params limits int 池最大水位
func WithLimits(limits int) optparams.Option[NewClientPoolOptions] {

	return optparams.NewFuncOption(
		func(o *NewClientPoolOptions) {
			o.Limits = limits
		})
}

// WithAcquireWaitTimeMS 创建池对象方法的参数,用于设置从连接池中获取连接的最大等待时长,单位ms.
// @params wait int 获取客户端等待时长
func WithAcquireWaitTimeMS(wait int) optparams.Option[NewClientPoolOptions] {
	return optparams.NewFuncOption(
		func(o *NewClientPoolOptions) {
			o.AcquireWaitTime = time.Duration(wait) * time.Millisecond
		})
}
