package grpcsdk

import (
	"github.com/Golang-Tools/optparams"
)

//AcquireOptions 设置Acquire方法的选项
type AcquireOptions struct {
	Force bool
}

//UntilEnd NewCtx方法的参数,用于设置ctx为不会超时
func Force() optparams.Option[AcquireOptions] {
	return optparams.NewFuncOption(
		func(o *AcquireOptions) {
			o.Force = true
		})
}
