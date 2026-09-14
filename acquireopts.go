package grpcsdk

import (
	"github.com/Golang-Tools/optparams"
)

// AcquireOptions 设置Acquire方法的选项
type AcquireOptions struct {
	//Force 是否强制获取客户端
	Force bool
}

// Force Acquire方法的参数,用于设置强制获取客户端,池中没有可用的客户端时会新建一个客户端对象
func Force() optparams.Option[AcquireOptions] {
	return optparams.NewFuncOption(
		func(o *AcquireOptions) {
			o.Force = true
		})
}
