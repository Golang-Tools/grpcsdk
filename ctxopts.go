package grpcsdk

import (
	"time"

	"github.com/Golang-Tools/optparams"
	"google.golang.org/grpc/metadata"
)

// CtxOptions 设置ctx行为的选项
type CtxOptions struct {
	//UntilEnd 是否让ctx没有超时时间
	UntilEnd bool
	//Timeout 请求超时,优先级高于SDKConfig中的Query_Timeout
	Timeout time.Duration
	//MetaData 请求携带的元数据
	MetaData metadata.MD
}

// UntilEnd NewCtx方法的参数,用于设置ctx为不会超时
func UntilEnd() optparams.Option[CtxOptions] {
	return optparams.NewFuncOption(
		func(o *CtxOptions) {
			o.UntilEnd = true
		})
}

// WithTimeout NewCtx方法的参数,用于设置ctx为指定的超时时长
// @params timeout time.Duration 请求超时,优先级高于SDKConfig中的Query_Timeout
func WithTimeout(timeout time.Duration) optparams.Option[CtxOptions] {
	return optparams.NewFuncOption(
		func(o *CtxOptions) {
			o.Timeout = timeout
		})
}

// WithRequestMeta NewCtx方法的参数,用于设置请求端信息到meta数据
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) WithRequestMeta() optparams.Option[CtxOptions] {
	return optparams.NewFuncOption(
		func(o *CtxOptions) {
			if o.MetaData == nil {
				o.MetaData = metadata.MD{}
			}
			if c.SDKConfig == nil {
				return
			}
			if c.SDKConfig.Requester_App_Name != "" {
				o.MetaData.Set("requester_app_name", c.SDKConfig.Requester_App_Name)
				if c.SDKConfig.Requester_App_Version != "" {
					o.MetaData.Set("requester_app_version", c.SDKConfig.Requester_App_Version)
				}
			}
		})
}

// WithMeta NewCtx方法的参数,用于设置信息到meta数据
// @params key string meta键
// @params value ...string meta值
func WithMeta(key string, value ...string) optparams.Option[CtxOptions] {
	return optparams.NewFuncOption(
		func(o *CtxOptions) {
			if o.MetaData == nil {
				o.MetaData = metadata.MD{}
			}
			o.MetaData.Set(key, value...)
		})
}
