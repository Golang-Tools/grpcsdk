package grpcsdk

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	log "github.com/Golang-Tools/loggerhelper/v2"
	"github.com/Golang-Tools/optparams"
	jsoniter "github.com/json-iterator/go"
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

var json = jsoniter.ConfigCompatibleWithStandardLibrary

//SDK 的客户端类型
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
type SDK[T any] struct {
	*SDKConfig
	opts                []grpc.DialOption
	callopts            []grpc.CallOption
	serviceconfig       map[string]interface{}
	addr                string
	factory             NewGrpcClientFunc[T]
	clientgetter        GrpcClientGetter[T]
	getClientGetterLock *sync.RWMutex
	Logger              *log.Log
}

//New 创建客户端对象
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@params factory NewGrpcClientFunc[T] 将grpc连接转化为grpc客户端的程序,可以在pb生成的模块中找到,通常以`NewXXXXXXClient`命名
//@params desc *grpc.ServiceDesc grpc服务的描述对象,可以在pb生成的模块中找到,通常以`XXXXX_ServiceDesc`命名
//@returns *SDK[T] SDK对象
func New[T any](factory NewGrpcClientFunc[T], desc *grpc.ServiceDesc) *SDK[T] {
	c := new(SDK[T])
	c.opts = []grpc.DialOption{}
	c.callopts = []grpc.CallOption{}
	c.serviceconfig = map[string]interface{}{}
	c.getClientGetterLock = &sync.RWMutex{}
	c.factory = factory
	c.SDKConfig = &SDKConfig{}
	log.Set(log.WithExtFields(log.Dict{"module": "grpcsdk", "target_service": desc.ServiceName}))
	c.Logger = log.Export()
	log.Set(log.WithExtFields(log.Dict{}))
	return c
}

//initMsgSize 初始化消息大小设置
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initMsgSize() {
	if c.Max_Recv_Msg_Size != 0 {
		c.callopts = append(c.callopts, grpc.MaxCallRecvMsgSize(c.Max_Recv_Msg_Size))
	}
	if c.Max_Send_Msg_Size != 0 {
		c.callopts = append(c.callopts, grpc.MaxCallSendMsgSize(c.Max_Send_Msg_Size))
	}

}

//initCompression 初始化压缩设置
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initCompression() {
	switch c.Compression {
	case "gzip":
		{
			c.callopts = append(c.callopts, grpc.UseCompressor(gzip.Name))
		}
	}
}

//initKeepalive 初始化keepalive的相关设置
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
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

//initPerformanceOpts 初始化连接的性能选项
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initPerformanceOpts() {
	if c.Conn_With_Block {
		c.opts = append(c.opts, grpc.WithBlock())
	}
	if c.Initial_Window_Size != 0 {
		c.opts = append(c.opts, grpc.WithInitialWindowSize(int32(c.Initial_Window_Size)))
	}
	if c.Initial_Conn_Window_Size != 0 {
		c.opts = append(c.opts, grpc.WithInitialConnWindowSize(int32(c.Initial_Conn_Window_Size)))
	}
}

//initTLS 初始化TLS设置
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initTLS() error {
	if c.Ca_Cert_Path != "" {
		if c.Client_Cert_Path != "" && c.Client_Key_Path != "" {
			cert, err := tls.LoadX509KeyPair(c.Client_Cert_Path, c.Client_Key_Path)
			if err != nil {
				c.Logger.Error("read client pem file error:", log.Dict{"err": err.Error(), "Cert_path": c.Client_Cert_Path, "Key_Path": c.Client_Key_Path})
				return err
			}
			capool := x509.NewCertPool()
			caCrt, err := ioutil.ReadFile(c.Ca_Cert_Path)
			if err != nil {
				c.Logger.Error("read ca pem file error:", log.Dict{"err": err.Error(), "path": c.Ca_Cert_Path})
				return err
			}
			capool.AppendCertsFromPEM(caCrt)
			tlsconf := &tls.Config{
				RootCAs:      capool,
				Certificates: []tls.Certificate{cert},
			}
			creds := credentials.NewTLS(tlsconf)
			c.opts = append(c.opts, grpc.WithTransportCredentials(creds))
		} else {
			creds, err := credentials.NewClientTLSFromFile(c.Ca_Cert_Path, "")
			if err != nil {
				c.Logger.Error("failed to load credentials", log.Dict{"err": err.Error()})
				return err
			}
			c.opts = append(c.opts, grpc.WithTransportCredentials(creds))
		}
	} else {
		c.opts = append(c.opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	return nil
}

//initWithoutBL 初始化没有堵在均衡设置的服务
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initWithoutBL() error {
	return c.initTLS()
}

//initWithDNSLB 初始化使用外部dns做负载均衡的设置
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initWithDNSLB() error {
	c.serviceconfig["loadBalancingPolicy"] = "round_robin"
	return c.initTLS()
}

//InitWithStandalone 初始化使用XDS协议做负载均衡的设置
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
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

//InitWithLocalBalance 初始化本地负载均衡的连接配置
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) initWithLocalBalance() error {
	serverName := ""
	if c.Requester_App_Name != "" {
		if c.Requester_App_Version != "" {
			serverName = fmt.Sprintf("%s-%s", c.Requester_App_Name, strings.ReplaceAll(c.Requester_App_Version, ".", "_"))
		} else {
			serverName = c.Requester_App_Name
		}
	}
	c.serviceconfig["loadBalancingPolicy"] = "round_robin"
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

//RegistInterceptor 注册拦截器
func (c *SDK[T]) RegistInterceptor() {
	if len(c.UnaryInterceptors) > 0 {
		for _, i := range c.UnaryInterceptors {
			c.opts = append(c.opts, grpc.WithUnaryInterceptor(i))
		}
	}
	if len(c.StreamInterceptors) > 0 {
		for _, i := range c.StreamInterceptors {
			c.opts = append(c.opts, grpc.WithStreamInterceptor(i))
		}
	}
}

//Init 初始化sdk客户端的连接信息
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
func (c *SDK[T]) Init(opts ...optparams.Option[SDKConfig]) error {
	optparams.GetOption(c.SDKConfig, opts...)
	if c.SDKConfig.Query_Addresses == nil {
		return errors.New("必须至少有一个地址")
	}
	switch len(c.SDKConfig.Query_Addresses) {
	case 0:
		{
			return errors.New("必须至少有一个地址")
		}
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
				err := c.initWithoutBL()
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
	c.RegistInterceptor()
	if len(c.serviceconfig) != 0 {
		serviceconfig, err := json.MarshalToString(c.serviceconfig)
		if err != nil {
			return err
		}
		c.opts = append(c.opts, grpc.WithDefaultServiceConfig(serviceconfig))
	}
	if len(c.callopts) != 0 {
		c.opts = append(c.opts, grpc.WithDefaultCallOptions(c.callopts...))
	}
	return nil
}

//NewCtx 创建请求的上下文,这个上下问可以带元数据信息
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@params opts ...optparams.Option[CtxOptions] 构造上下文的参数
//@returns ctx context.Context 上下文对象
//@returns cancel context.CancelFunc 上下文取消函数
func (c *SDK[T]) NewCtx(opts ...optparams.Option[CtxOptions]) (ctx context.Context, cancel context.CancelFunc) {
	dopt := CtxOptions{}
	optparams.GetOption(&dopt, opts...)
	if dopt.UntilEnd || c.SDKConfig.Query_Timeout <= 0 {
		ctx, cancel = context.WithCancel(context.Background())
	} else {
		var timeout time.Duration
		if dopt.Timeout > 0 {
			timeout = dopt.Timeout
		} else {
			timeout = time.Duration(c.SDKConfig.Query_Timeout) * time.Millisecond
		}
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	}
	if len(dopt.MetaData) > 0 {
		ctx = metadata.NewOutgoingContext(ctx, dopt.MetaData)
	}
	return
}

//GetClient 返回接口和回收函数
//注意如果获取连接时报错会报panic
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@params opts ...optparams.Option[AcquireOptions] acquire方法的参数,只有`Force()`可用,表示是否强制获取,如果队列中已经没有可用的客户端了则创建一个客户端对象
//@returns T grpc客户端对象
//@returns ReleaseFunc grpc客户端回收函数
func (c *SDK[T]) GetClient(opts ...optparams.Option[AcquireOptions]) (T, ReleaseFunc) {
	c.getClientGetterLock.RLock()
	clientgetter := c.clientgetter
	c.getClientGetterLock.RUnlock()
	var err error
	if clientgetter == nil {
		c.Logger.Debug("new clientgetter")
		clientgetter, err = c.NewClientGetter()
		if err != nil {
			c.Logger.Error("NewClientGetter get err", log.Dict{"err": err.Error()})
			panic(err)
		}
	}
	if c.Client_Pool {
		c.Logger.Debug("clientgetter is pool")
	} else {
		c.Logger.Debug("clientgetter is client")
	}
	client, err := clientgetter.Acquire(opts...)
	if err != nil {
		c.Logger.Error("Acquire get err", log.Dict{"err": err.Error()})
		panic(err)
	}
	res := client.AsGrpcClient()
	if c.Client_Pool {
		return res, func() { clientgetter.Release(client) }
	}
	return res, func() {}
}

//newClientGetter 建立一个新的客户端获取器
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@returns GrpcClientGetter[T] 客户端获取器
//@returns error 错误信息
func (c *SDK[T]) newClientGetter() (GrpcClientGetter[T], error) {
	if c.Client_Pool {
		opts := []optparams.Option[NewClientPoolOptions]{HasClientConfig(WithAddr(c.addr), WithDialOpts(c.opts...))}
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
	} else {
		cli, err := NewClient(c.factory, WithClientConfig(&NewClientOptions{Addr: c.addr, DialOpts: c.opts}))
		if err != nil {
			return nil, err
		}
		return cli, nil
	}
}

//NewClientGetter 建立一个新的客户端获取器,并绑定至sdk中
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@returns GrpcClientGetter[T] 客户端获取器
//@returns error 错误信息
func (c *SDK[T]) NewClientGetter() (GrpcClientGetter[T], error) {
	clientgetter, err := c.newClientGetter()
	if err != nil {
		return nil, err
	}
	c.getClientGetterLock.Lock()
	defer c.getClientGetterLock.Unlock()
	c.clientgetter = clientgetter
	return clientgetter, nil
}

//NewClientGetter 建立一个新的客户端获取器,并绑定至sdk中
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
//@returns error 错误信息
func (c *SDK[T]) Close() error {
	if c.clientgetter != nil {
		return c.clientgetter.Close()
	}
	return nil
}
