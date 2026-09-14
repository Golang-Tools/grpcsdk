package grpcsdk

import (
	"github.com/Golang-Tools/optparams"
	grpc "google.golang.org/grpc"
)

// SDKConfig 的客户端类型
type SDKConfig struct {
	Query_Addresses       []string `json:"query_addresses" jsonschema:"required,description=连接服务的主机地址"`
	Requester_App_Name    string   `json:"requester_app_name,omitempty" jsonschema:"description=请求方服务名"`
	Requester_App_Version string   `json:"requester_app_version,omitempty" jsonschema:"description=请求方服务版本"`

	// 性能设置
	Initial_Window_Size                         int  `json:"initial_window_size,omitempty" jsonschema:"description=基于Stream的滑动窗口大小"`
	Initial_Conn_Window_Size                    int  `json:"initial_conn_window_size,omitempty" jsonschema:"description=基于Connection的滑动窗口大小"`
	Keepalive_Time                              int  `json:"keepalive_time,omitempty" jsonschema:"description=空闲连接每隔n秒ping一次客户端已确保连接存活"`
	Keepalive_Timeout                           int  `json:"keepalive_timeout,omitempty" jsonschema:"description=ping时长超过n则认为连接已死"`
	Keepalive_Enforcement_Permit_Without_Stream bool `json:"keepalive_enforement_permit_without_stream,omitempty" jsonschema:"description=是否当连接空闲时仍然发送PING帧监测"`
	Conn_With_Block                             bool `json:"conn_with_block,omitempty" jsonschema:"description=建立连接时阻塞等待连接就绪,默认最长等待10s"`
	Max_Recv_Msg_Size                           int  `json:"max_rec_msg_size,omitempty" jsonschema:"description=允许接收的最大消息长度"`
	Max_Send_Msg_Size                           int  `json:"max_send_msg_size,omitempty" jsonschema:"description=允许发送的最大消息长度"`

	//压缩设置,目前只支持gzip
	Compression string `json:"compression,omitempty" jsonschema:"description=使用哪种方式压缩发送的消息,enum=gzip"`

	// TLS设置
	Ca_Cert_Path     string `json:"ca_cert_path,omitempty" jsonschema:"description=如果要使用tls则需要指定根证书位置"`
	Client_Cert_Path string `json:"client_cert_path,omitempty" jsonschema:"description=客户端证书位置"`
	Client_Key_Path  string `json:"client_key_path,omitempty" jsonschema:"description=客户端证书对应的私钥位置"`

	// XDS设置
	XDS_CREDS bool `json:"xds_creds,omitempty" jsonschema:"description=当address的schema是xds时是否使用xds的令牌加密访问"`

	//客户端连接池
	Client_Pool                      bool `json:"client_pool" jsonschema:"description=是否使用grpc的客户端池"`
	Client_Pool_Reservations         int  `json:"client_pool_reservations" jsonschema:"description=使用客户端池时的池注水水位"`
	Client_Pool_Limits               int  `json:"client_pool_limits" jsonschema:"description=使用客户端池时的池最大水位"`
	Client_Pool_Acquire_Wait_Time_MS int  `json:"client_pool_acquire_wait_time_ms" jsonschema:"description=获取客户端池时的最大等待时间"`
	// 请求超时设置
	Query_Timeout int `json:"query_timeout,omitempty" jsonschema:"description=请求服务的最大超时时间单位ms"`

	UnaryInterceptors  []grpc.UnaryClientInterceptor  `json:"-" jsonschema:"nullable"`
	StreamInterceptors []grpc.StreamClientInterceptor `json:"-" jsonschema:"nullable"`
}

// WithConfig sdk.Init方法的参数,用于通过SDKConfig对象设置sdk的全部配置
// @params config *SDKConfig 要设置的配置对象,内部会拷贝一份,不会与外部共享
func WithConfig(config *SDKConfig) optparams.Option[SDKConfig] { //<- 2.定义可用的关键字参数项,一般命名上使用`with`开头
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			if config == nil {
				return
			}
			*o = *config
		})
}

// WithQueryAddresses sdk.Init方法的参数,用于设置sdk请求的地址
// @params addresses ...string 连接服务的主机地址
func WithQueryAddresses(addresses ...string) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Query_Addresses = addresses
		})
}

// WithRequesterAppName sdk.Init方法的参数,用于设置sdk请求方服务名
// @params name string 请求端app名
func WithRequesterAppName(name string) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Requester_App_Name = name
		})
}

// WithRequesterAppVersion sdk.Init方法的参数,用于设置sdk请求方服务版本
// @params version string 请求方服务版本
func WithRequesterAppVersion(version string) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Requester_App_Version = version
		})
}

// WithInitialWindowSize sdk.Init方法的参数,用于设置sdk基于Stream的滑动窗口大小
// @params size int 基于Stream的滑动窗口大小
func WithInitialWindowSize(size int) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Initial_Window_Size = size
		})
}

// 性能设置

// WithInitialConnWindowSize sdk.Init方法的参数,用于设置sdk基于Connection的滑动窗口大小
// @params size int 基于Connection的滑动窗口大小
func WithInitialConnWindowSize(size int) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Initial_Conn_Window_Size = size
		})
}

// WithKeepaliveTime sdk.Init方法的参数,用于设置sdk空闲连接每隔n秒ping一次客户端已确保连接存活
// @params alivetime int 空闲连接每隔n秒ping一次客户端已确保连接存活,单位秒
func WithKeepaliveTime(alivetime int) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Keepalive_Time = alivetime
		})
}

// WithKeepaliveTimeout sdk.Init方法的参数,用于设置sdkping时长超过n则认为连接已死
// @params timeout int ping时长超过n则认为连接已死,单位秒
func WithKeepaliveTimeout(timeout int) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Keepalive_Timeout = timeout
		})
}

// WithKeepaliveEnforcementPermitWithoutStream sdk.Init方法的参数,用于设置sdk是否当连接空闲时仍然发送PING帧监测
func WithKeepaliveEnforcementPermitWithoutStream() optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Keepalive_Enforcement_Permit_Without_Stream = true
		})
}

// WithConnWithBlock sdk.Init方法的参数,用于设置sdk是否同步的建立连接
func WithConnWithBlock() optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Conn_With_Block = true
		})
}

// WithMaxRecvMsgSize sdk.Init方法的参数,用于设置sdk允许接收的最大消息长度
// @params size int 允许接收的最大消息长度
func WithMaxRecvMsgSize(size int) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Max_Recv_Msg_Size = size
		})
}

// WithMaxSendMsgSize sdk.Init方法的参数,用于设置sdk允许发送的最大消息长度
// @params size int 允许发送的最大消息长度
func WithMaxSendMsgSize(size int) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Max_Send_Msg_Size = size
		})
}

// WithCompression sdk.Init方法的参数,用于设置sdk使用哪种方式压缩发送的消息
// @params protocol string 协议名,目前可选的只有gzip
func WithCompression(protocol string) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Compression = protocol
		})
}

// WithCaCertPath sdk.Init方法的参数,用于设置sdk如果要使用tls则需要指定根证书位置
// @params path string 根证书路径
func WithCaCertPath(path string) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Ca_Cert_Path = path
		})
}

// WithClientCertPath sdk.Init方法的参数,用于设置sdk客户端证书位置
// @params path string 客户端证书路径
func WithClientCertPath(path string) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Client_Cert_Path = path
		})
}

// WithClientKeyPath sdk.Init方法的参数,用于设置sdk客户端证书对应的私钥位置
// @params path string 客户端证书对应的私钥路径
func WithClientKeyPath(path string) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Client_Key_Path = path
		})
}

// WithXDSCREDS sdk.Init方法的参数,用于设置sdk当address的schema是xds时是否使用xds的令牌加密访问
func WithXDSCREDS() optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.XDS_CREDS = true
		})
}

// WithClientPool sdk.Init方法的参数,用于设置sdk是否使用grpc的客户端池
func WithClientPool() optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Client_Pool = true
		})
}

// WithClientPoolReservations sdk.Init方法的参数,用于设置sdk使用客户端池时的池注水水位
// @params n int 使用客户端池时的池注水水位
func WithClientPoolReservations(n int) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Client_Pool_Reservations = n
		})
}

// WithClientPoolLimits sdk.Init方法的参数,用于设置sdk使用客户端池时的池最大水位
// @params n int 使用客户端池时的池最大水位
func WithClientPoolLimits(n int) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Client_Pool_Limits = n
		})
}

// WithClientPoolAcquireWaitTime sdk.Init方法的参数,用于设置sdk获取客户端池时的最大等待时间
// @params wait int 获取客户端池时的最大等待时间,单位ms
func WithClientPoolAcquireWaitTime(wait int) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Client_Pool_Acquire_Wait_Time_MS = wait
		})
}

// WithQueryTimeout sdk.Init方法的参数,用于设置sdk请求服务的最大超时时间
// @params wait int 请求服务的最大超时时间,单位ms
func WithQueryTimeout(timeout int) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Query_Timeout = timeout
		})
}

// WithUnaryInterceptors sdk.Init方法的参数,用于设置sdk的请求拦截器
// @params interceptor ...grpc.UnaryClientInterceptor 请求拦截器
func WithUnaryInterceptors(interceptor ...grpc.UnaryClientInterceptor) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.UnaryInterceptors = interceptor
		})
}

// WithStreamInterceptors sdk.Init方法的参数,用于设置sdk的流请求拦截器
// @params interceptor ...grpc.StreamClientInterceptor 流请求拦截器
func WithStreamInterceptors(interceptor ...grpc.StreamClientInterceptor) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.StreamInterceptors = interceptor
		})
}
