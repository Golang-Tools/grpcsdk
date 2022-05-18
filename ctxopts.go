package grpcsdk

import (
	"time"

	"github.com/Golang-Tools/optparams"
	"google.golang.org/grpc/metadata"
)

//CtxOptions 设置ctx行为的选项
type CtxOptions struct {
	UntilEnd bool
	Timeout  time.Duration
	MetaData metadata.MD
}

//UntilEnd NewCtx方法的参数,用于设置ctx为不会超时
func UntilEnd() optparams.Option[CtxOptions] {
	return optparams.NewFuncOption(
		func(o *CtxOptions) {
			o.UntilEnd = true
		})
}

//WithTimeout NewCtx方法的参数,用于设置ctx为指定的超时时长
//@params timeout time.Duration 请求超时,单位ms
func WithTimeout(timeout time.Duration) optparams.Option[CtxOptions] {
	return optparams.NewFuncOption(
		func(o *CtxOptions) {
			o.Timeout = timeout
		})
}

//WithRequestMeta NewCtx方法的参数,用于设置请求端信息到meta数据
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) WithRequestMeta() optparams.Option[CtxOptions] {
	return optparams.NewFuncOption(
		func(o *CtxOptions) {
			if o.MetaData == nil {
				o.MetaData = metadata.MD{}
			}
			if c.Requester_App_Name != "" {
				o.MetaData.Set("requester_app_name", c.Requester_App_Name)
				if c.Requester_App_Version != "" {
					o.MetaData.Set("requester_app_version", c.Requester_App_Version)
				}
			}
		})
}

//WithMeta NewCtx方法的参数,用于设置信息到meta数据
//@params key string meta键
//@params value ...string meta值
func WithMeta(key string, value ...string) optparams.Option[CtxOptions] {
	return optparams.NewFuncOption(
		func(o *CtxOptions) {
			if o.MetaData == nil {
				o.MetaData = metadata.MD{}
			}
			o.MetaData.Set(key, value...)
		})
}
