package grpcsdk

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/Golang-Tools/loggerhelper/v4"
	"github.com/Golang-Tools/optparams"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	xdscreds "google.golang.org/grpc/credentials/xds"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	resolver "google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	_ "google.golang.org/grpc/xds"
)

// SDK 的客户端类型
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
type SDK[T any] struct {
	*SDKConfig
	opts                []grpc.DialOption
	callopts            []grpc.CallOption
	serviceconfig       map[string]interface{}
	addr                string
	factory             NewGrpcClientFunc[T]
	clientgetter        GrpcClientGetter[T]
	getClientGetterLock *sync.RWMutex

	//Logger 带有`module`和`target_service`字段的日志对象
	//它基于创建时刻的全局日志配置输出,日志等级会跟随后续log.Set的设置变化
	Logger *slog.Logger
}

// New 创建客户端对象
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
// @params factory NewGrpcClientFunc[T] 将grpc连接转化为grpc客户端的程序,可以在pb生成的模块中找到,通常以`NewXXXXXXClient`命名
// @params desc *grpc.ServiceDesc grpc服务的描述对象,可以在pb生成的模块中找到,通常以`XXXXX_ServiceDesc`命名
// @returns *SDK[T] SDK对象
func New[T any](factory NewGrpcClientFunc[T], desc *grpc.ServiceDesc) *SDK[T] {
	c := new(SDK[T])
	c.opts = []grpc.DialOption{}
	c.callopts = []grpc.CallOption{}
	c.serviceconfig = map[string]interface{}{}
	c.getClientGetterLock = &sync.RWMutex{}
	c.factory = factory
	c.SDKConfig = &SDKConfig{}
	fields := []any{"module", "grpcsdk"}
	if desc != nil {
		fields = append(fields, "target_service", desc.ServiceName)
	}
	c.Logger = log.GetLogger().With(fields...)
	return c
}

// initMsgSize 初始化消息大小设置
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initMsgSize() {
	if c.Max_Recv_Msg_Size != 0 {
		c.callopts = append(c.callopts, grpc.MaxCallRecvMsgSize(c.Max_Recv_Msg_Size))
	}
	if c.Max_Send_Msg_Size != 0 {
		c.callopts = append(c.callopts, grpc.MaxCallSendMsgSize(c.Max_Send_Msg_Size))
	}

}

// initCompression 初始化压缩设置
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initCompression() {
	switch c.Compression {
	case "gzip":
		{
			c.callopts = append(c.callopts, grpc.UseCompressor(gzip.Name))
		}
	}
}

// initKeepalive 初始化keepalive的相关设置
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initKeepalive() {
	if c.Keepalive_Time != 0 || c.Keepalive_Timeout != 0 || c.Keepalive_Enforcement_Permit_Without_Stream {
		kacp := keepalive.ClientParameters{
			Time:                time.Duration(c.Keepalive_Time) * time.Second,
			Timeout:             time.Duration(c.Keepalive_Timeout) * time.Second,
			PermitWithoutStream: c.Keepalive_Enforcement_Permit_Without_Stream, // send pings even without active streams
		}
		c.opts = append(c.opts, grpc.WithKeepaliveParams(kacp))
	}
}

// initPerformanceOpts 初始化连接的性能选项
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initPerformanceOpts() {
	// StaticMethod标记调用来自编译期确定的方法,grpc的观测插件(如stats/opentelemetry)可以因此把方法名作为指标的属性
	c.callopts = append(c.callopts, grpc.StaticMethod())
	if c.Initial_Window_Size != 0 {
		c.opts = append(c.opts, grpc.WithInitialWindowSize(int32(c.Initial_Window_Size)))
	}
	if c.Initial_Conn_Window_Size != 0 {
		c.opts = append(c.opts, grpc.WithInitialConnWindowSize(int32(c.Initial_Conn_Window_Size)))
	}
}

// initConnectParams 初始化连接参数与空闲连接回收设置
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initConnectParams() {
	if c.Connect_Params != nil {
		c.opts = append(c.opts, grpc.WithConnectParams(*c.Connect_Params))
	}
	switch {
	case c.Idle_Timeout_MS > 0:
		c.opts = append(c.opts, grpc.WithIdleTimeout(time.Duration(c.Idle_Timeout_MS)*time.Millisecond))
	case c.Idle_Timeout_MS < 0:
		// grpc的WithIdleTimeout传入0表示禁用空闲连接回收
		c.opts = append(c.opts, grpc.WithIdleTimeout(0))
	}
}

// initRetryPolicy 初始化重试策略,策略会写入service config的默认methodConfig
// retryPolicy只允许配置在methodConfig中,空name表示对所有方法生效
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
// @returns error 错误信息
func (c *SDK[T]) initRetryPolicy() error {
	if c.Retry_Policy == nil {
		return nil
	}
	if err := c.Retry_Policy.validate(); err != nil {
		return err
	}
	c.serviceconfig["methodConfig"] = []interface{}{c.Retry_Policy.toMethodConfig()}
	return nil
}

// loadBalancingPolicy 返回使用的负载均衡策略,未显式设置时使用传入的默认策略
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
// @params defaultpolicy string 默认策略
// @returns string 负载均衡策略
func (c *SDK[T]) loadBalancingPolicy(defaultpolicy string) string {
	if c.Load_Balancing_Policy != "" {
		return c.Load_Balancing_Policy
	}
	return defaultpolicy
}

// initTLS 初始化TLS设置
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initTLS() error {
	if c.Ca_Cert_Path != "" {
		caCrt, err := os.ReadFile(c.Ca_Cert_Path)
		if err != nil {
			c.Logger.Error("read ca pem file error", "err", err.Error(), "path", c.Ca_Cert_Path)
			return err
		}
		capool := x509.NewCertPool()
		if !capool.AppendCertsFromPEM(caCrt) {
			c.Logger.Error("no valid certificate parsed from ca file", "path", c.Ca_Cert_Path)
			return ErrCACertNotParsed
		}
		tlsconf := &tls.Config{
			RootCAs:    capool,
			MinVersion: tls.VersionTLS12,
		}
		if c.Client_Cert_Path != "" && c.Client_Key_Path != "" {
			cert, err := tls.LoadX509KeyPair(c.Client_Cert_Path, c.Client_Key_Path)
			if err != nil {
				c.Logger.Error("read client pem file error", "err", err.Error(), "Cert_path", c.Client_Cert_Path, "Key_Path", c.Client_Key_Path)
				return err
			}
			tlsconf.Certificates = []tls.Certificate{cert}
		}
		c.opts = append(c.opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsconf)))
	} else {
		c.opts = append(c.opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	return nil
}

// initWithoutLB 初始化没有负载均衡设置的服务
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initWithoutLB() error {
	if c.Load_Balancing_Policy != "" {
		c.serviceconfig["loadBalancingPolicy"] = c.Load_Balancing_Policy
	}
	return c.initTLS()
}

// initWithDNSLB 初始化使用外部dns做负载均衡的设置
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initWithDNSLB() error {
	c.serviceconfig["loadBalancingPolicy"] = c.loadBalancingPolicy("round_robin")
	return c.initTLS()
}

// initWithXDSLB 初始化使用XDS协议做负载均衡的设置
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initWithXDSLB() error {
	creds := insecure.NewCredentials()
	var err error
	if c.XDS_CREDS {
		creds, err = xdscreds.NewClientCredentials(xdscreds.ClientOptions{FallbackCreds: insecure.NewCredentials()})
	}
	if err != nil {
		return err
	}
	c.opts = append(c.opts, grpc.WithTransportCredentials(creds))
	return nil
}

// initWithLocalBalance 初始化本地负载均衡的连接配置
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initWithLocalBalance() error {
	serverName := ""
	if c.Requester_App_Name != "" {
		if c.Requester_App_Version != "" {
			serverName = fmt.Sprintf("%s-%s", c.Requester_App_Name, strings.ReplaceAll(c.Requester_App_Version, ".", "_"))
		} else {
			serverName = c.Requester_App_Name
		}
	}
	c.serviceconfig["loadBalancingPolicy"] = c.loadBalancingPolicy("round_robin")
	c.serviceconfig["healthCheckConfig"] = map[string]string{"serviceName": serverName}

	r := manual.NewBuilderWithScheme("localbalancer")
	addresses := []resolver.Address{}
	for _, addr := range c.Query_Addresses {
		addresses = append(addresses, resolver.Address{Addr: addr})
	}
	r.InitialState(resolver.State{
		Addresses: addresses,
	})
	c.addr = fmt.Sprintf("%s:///%s", r.Scheme(), serverName)
	c.opts = append(c.opts, grpc.WithResolvers(r))
	return c.initTLS()
}

// RegistInterceptor 注册拦截器,多个拦截器会按顺序串联
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) RegistInterceptor() {
	if len(c.UnaryInterceptors) > 0 {
		c.opts = append(c.opts, grpc.WithChainUnaryInterceptor(c.UnaryInterceptors...))
	}
	if len(c.StreamInterceptors) > 0 {
		c.opts = append(c.opts, grpc.WithChainStreamInterceptor(c.StreamInterceptors...))
	}
}

// Init 初始化sdk客户端的连接信息,重复调用时会关闭旧的客户端获取器并按新的配置重建
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
// @params opts ...optparams.Option[SDKConfig] 初始化使用的配置项,详细可以看config.go文件
// @returns error 错误信息
func (c *SDK[T]) Init(opts ...optparams.Option[SDKConfig]) error {
	// 重置构造连接使用的状态,保证Init重复调用时不会累积指令
	c.opts = []grpc.DialOption{}
	c.callopts = []grpc.CallOption{}
	c.serviceconfig = map[string]interface{}{}

	// 这里用Apply,选项的修改会直接作用到c.SDKConfig上
	optparams.Apply(c.SDKConfig, opts...)
	if len(c.Query_Addresses) == 0 {
		return ErrNoAddresses
	}
	switch len(c.Query_Addresses) {
	case 1:
		{
			c.addr = c.Query_Addresses[0]
			if strings.HasPrefix(c.addr, "dns:///") {
				err := c.initWithDNSLB()
				if err != nil {
					return err
				}
			} else if strings.HasPrefix(c.addr, "xds:///") {
				err := c.initWithXDSLB()
				if err != nil {
					return err
				}
			} else {
				err := c.initWithoutLB()
				if err != nil {
					return err
				}
			}
		}
	default:
		{
			err := c.initWithLocalBalance()
			if err != nil {
				return err
			}
		}
	}
	c.initMsgSize()
	c.initCompression()
	c.initKeepalive()
	c.initPerformanceOpts()
	c.initConnectParams()
	if err := c.initRetryPolicy(); err != nil {
		return err
	}
	c.RegistInterceptor()
	if len(c.serviceconfig) != 0 {
		serviceconfig, err := json.Marshal(c.serviceconfig)
		if err != nil {
			return err
		}
		c.opts = append(c.opts, grpc.WithDefaultServiceConfig(string(serviceconfig)))
	}
	if len(c.callopts) != 0 {
		c.opts = append(c.opts, grpc.WithDefaultCallOptions(c.callopts...))
	}

	// 已经建立过客户端获取器时先关闭,避免旧连接泄漏
	c.getClientGetterLock.Lock()
	oldgetter := c.clientgetter
	c.clientgetter = nil
	c.getClientGetterLock.Unlock()
	if oldgetter != nil {
		_ = oldgetter.Close()
	}
	return nil
}

// NewCtx 创建请求的上下文,这个上下文可以带元数据信息
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
// @params opts ...optparams.Option[CtxOptions] 构造上下文的参数
// @returns ctx context.Context 上下文对象
// @returns cancel context.CancelFunc 上下文取消函数
func (c *SDK[T]) NewCtx(opts ...optparams.Option[CtxOptions]) (ctx context.Context, cancel context.CancelFunc) {
	dopt := CtxOptions{}
	optparams.Apply(&dopt, opts...)
	timeout := dopt.Timeout
	if timeout <= 0 && c.SDKConfig.Query_Timeout > 0 {
		// 没有显式设置超时时间时使用SDK配置中的超时时间
		timeout = time.Duration(c.SDKConfig.Query_Timeout) * time.Millisecond
	}
	if !dopt.UntilEnd && timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	if len(dopt.MetaData) > 0 {
		ctx = metadata.NewOutgoingContext(ctx, dopt.MetaData)
	}
	return
}

// GetClient 返回接口和回收函数
// 注意如果获取连接时报错会报panic
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
// @params opts ...optparams.Option[AcquireOptions] acquire方法的参数,只有`Force()`可用,表示是否强制获取,如果队列中已经没有可用的客户端了则创建一个客户端对象
// @returns T grpc客户端对象
// @returns ReleaseFunc grpc客户端回收函数
func (c *SDK[T]) GetClient(opts ...optparams.Option[AcquireOptions]) (T, ReleaseFunc) {
	clientgetter, err := c.NewClientGetter()
	if err != nil {
		c.Logger.Error("NewClientGetter get err", "err", err.Error())
		panic(err)
	}
	client, err := clientgetter.Acquire(opts...)
	if err != nil {
		c.Logger.Error("Acquire get err", "err", err.Error())
		panic(err)
	}
	res := client.AsGrpcClient()
	// 单客户端模式下Release是空操作,池模式下会按安全水位回收连接
	return res, func() { clientgetter.Release(client) }
}

// newClientGetter 建立一个新的客户端获取器
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
// @returns GrpcClientGetter[T] 客户端获取器
// @returns error 错误信息
func (c *SDK[T]) newClientGetter() (GrpcClientGetter[T], error) {
	blockopts := []optparams.Option[NewClientOptions]{}
	if c.Conn_With_Block {
		// 这里用WithBlockUntilReady替代已经废弃的grpc.WithBlock,建连时阻塞等待连接就绪
		blockopts = append(blockopts, WithBlockUntilReady())
	}
	if c.Client_Pool {
		cliopts := append([]optparams.Option[NewClientOptions]{WithAddr(c.addr), WithDialOpts(c.opts...)}, blockopts...)
		opts := []optparams.Option[NewClientPoolOptions]{HasClientConfig(cliopts...)}
		if c.Client_Pool_Acquire_Wait_Time_MS > 0 {
			opts = append(opts, WithAcquireWaitTimeMS(c.Client_Pool_Acquire_Wait_Time_MS))
		}
		if c.Client_Pool_Limits > 0 {
			opts = append(opts, WithLimits(c.Client_Pool_Limits))
		}
		if c.Client_Pool_Reservations > 0 {
			opts = append(opts, WithReservations(c.Client_Pool_Reservations))
		}
		pool, err := NewPool(c.factory, opts...)
		if err != nil {
			return nil, err
		}
		return pool, nil
	}
	cliopts := append([]optparams.Option[NewClientOptions]{WithClientConfig(&NewClientOptions{Addr: c.addr, DialOpts: c.opts})}, blockopts...)
	cli, err := NewClient(c.factory, cliopts...)
	if err != nil {
		return nil, err
	}
	return cli, nil
}

// NewClientGetter 获取客户端的获取器,首次调用时创建并绑定至sdk中,重复调用返回已经创建的对象
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
// @returns GrpcClientGetter[T] 客户端获取器
// @returns error 错误信息
func (c *SDK[T]) NewClientGetter() (GrpcClientGetter[T], error) {
	c.getClientGetterLock.RLock()
	if c.clientgetter != nil {
		getter := c.clientgetter
		c.getClientGetterLock.RUnlock()
		return getter, nil
	}
	c.getClientGetterLock.RUnlock()

	c.getClientGetterLock.Lock()
	defer c.getClientGetterLock.Unlock()
	// 双重检查,避免并发调用时重复创建
	if c.clientgetter != nil {
		return c.clientgetter, nil
	}
	clientgetter, err := c.newClientGetter()
	if err != nil {
		return nil, err
	}
	c.clientgetter = clientgetter
	return clientgetter, nil
}

// Close 关闭客户端获取器并释放其持有的连接,重复调用是安全的
// 关闭后如果再次调用GetClient会按当前配置重新建立连接
// @generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
// @returns error 错误信息
func (c *SDK[T]) Close() error {
	c.getClientGetterLock.Lock()
	clientgetter := c.clientgetter
	c.clientgetter = nil
	c.getClientGetterLock.Unlock()
	if clientgetter != nil {
		return clientgetter.Close()
	}
	return nil
}
