package grpcsdk

import (
	"fmt"
	"strconv"
	"time"

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

	// 连接设置
	Connect_Params        *grpc.ConnectParams `json:"-" jsonschema:"nullable"`
	Idle_Timeout_MS       int                 `json:"idle_timeout_ms,omitempty" jsonschema:"description=连接空闲回收时长,单位ms,0使用grpc默认的30分钟,负数表示禁用空闲回收"`
	Load_Balancing_Policy string              `json:"load_balancing_policy,omitempty" jsonschema:"description=负载均衡策略,可选pick_first/round_robin/least_request/weighted_round_robin等,留空时按地址自动选择"`

	// 重试设置
	Retry_Policy *RetryPolicy `json:"retry_policy,omitempty" jsonschema:"description=grpc内建重试策略,只有幂等的方法才可以配置"`

	UnaryInterceptors  []grpc.UnaryClientInterceptor  `json:"-" jsonschema:"nullable"`
	StreamInterceptors []grpc.StreamClientInterceptor `json:"-" jsonschema:"nullable"`
}

// RetryPolicy grpc内建的重试策略,对应service config中的retryPolicy
type RetryPolicy struct {
	//MaxAttempts 最大尝试次数(包含首次请求),取值范围[2,5]
	MaxAttempts int `json:"max_attempts" jsonschema:"description=最大尝试次数(包含首次请求),取值范围2到5"`
	//InitialBackoff 首次重试前的等待时长
	InitialBackoff time.Duration `json:"initial_backoff" jsonschema:"description=首次重试前的等待时长"`
	//MaxBackoff 重试等待时长的上限
	MaxBackoff time.Duration `json:"max_backoff" jsonschema:"description=重试等待时长的上限"`
	//BackoffMultiplier 退避倍数
	BackoffMultiplier float64 `json:"backoff_multiplier" jsonschema:"description=退避倍数"`
	//RetryableStatusCodes 可以重试的状态码,如"UNAVAILABLE"、"RESOURCE_EXHAUSTED"
	RetryableStatusCodes []string `json:"retryable_status_codes" jsonschema:"description=可以重试的状态码"`
}

// toMethodConfig 转换为service config中默认methodConfig的重试策略
// retryPolicy只允许配置在methodConfig中,空name表示对所有方法生效
// @returns map[string]interface{} service config中的默认methodConfig
func (p *RetryPolicy) toMethodConfig() map[string]interface{} {
	return map[string]interface{}{
		"name": []interface{}{map[string]interface{}{}},
		"retryPolicy": map[string]interface{}{
			"maxAttempts":          p.MaxAttempts,
			"initialBackoff":       durationToServiceConfig(p.InitialBackoff),
			"maxBackoff":           durationToServiceConfig(p.MaxBackoff),
			"backoffMultiplier":    p.BackoffMultiplier,
			"retryableStatusCodes": p.RetryableStatusCodes,
		},
	}
}

// validate 校验重试策略参数是否合法
// @returns error 错误信息
func (p *RetryPolicy) validate() error {
	if p.MaxAttempts < 2 || p.MaxAttempts > 5 {
		return fmt.Errorf("%w: maxAttempts must be in [2,5], got %d", ErrInvalidRetryPolicy, p.MaxAttempts)
	}
	if len(p.RetryableStatusCodes) == 0 {
		return fmt.Errorf("%w: retryableStatusCodes must not be empty", ErrInvalidRetryPolicy)
	}
	return nil
}

// durationToServiceConfig 把时长转换为service config中protobuf Duration的JSON格式(如"0.1s")
// @params d time.Duration 要转换的时长
// @returns string service config中的时长
func durationToServiceConfig(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', -1, 64) + "s"
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

// WithConnectParams sdk.Init方法的参数,用于设置连接建立与维护的参数(重连退避与单次建连最短超时)
// @params params grpc.ConnectParams 连接参数
func WithConnectParams(params grpc.ConnectParams) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Connect_Params = &params
		})
}

// WithIdleTimeoutMS sdk.Init方法的参数,用于设置连接空闲回收时长
// @params timeout int 空闲回收时长,单位ms;0使用grpc默认的30分钟,负数表示禁用空闲回收
func WithIdleTimeoutMS(timeout int) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Idle_Timeout_MS = timeout
		})
}

// WithLoadBalancingPolicy sdk.Init方法的参数,用于设置负载均衡策略
// 单地址留空时使用pick_first;多地址与dns地址留空时使用round_robin;xds地址的负载均衡由xds配置决定
// @params policy string 策略名,可选pick_first/round_robin/least_request/weighted_round_robin等
func WithLoadBalancingPolicy(policy string) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Load_Balancing_Policy = policy
		})
}

// WithRetryPolicy sdk.Init方法的参数,用于设置grpc内建的重试策略
// 注意重试会重复发送请求,只有幂等的方法才可以配置重试
// @params policy *RetryPolicy 重试策略
func WithRetryPolicy(policy *RetryPolicy) optparams.Option[SDKConfig] {
	return optparams.NewFuncOption(
		func(o *SDKConfig) {
			o.Retry_Policy = policy
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
