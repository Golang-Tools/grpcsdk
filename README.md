# grpcsdk

grpc的客户端sdk模板,使用它快速构造grpc的sdk

本项目只适用于go 1.25+;模块路径为`github.com/Golang-Tools/grpcsdk/v2`(旧路径`github.com/Golang-Tools/grpcsdk`停留在v0.0.2不再更新);
日志使用标准库`log/slog`(由`github.com/Golang-Tools/loggerhelper/v4`提供),关键字参数使用`github.com/Golang-Tools/optparams` v1.0.0.

## 使用步骤

1. 将proto文件转成go模块

    `protoc -I xxx --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative --go_out=xxx --go-grpc_out=xxx xxx.proto"`

2. 在生成的文件中找到客户端接口(以`XXXXClient`命名的`interface`)
3. 在生成的文件中找到服务描述对象(以`XXXXX_ServiceDesc`命名的变量)
4. 使用`func New[T any](factory NewGrpcClientFunc[T], desc *grpc.ServiceDesc) *SDK[T]`创建一个sdk实例
5. 使用`func (c *SDK[T]) Init(opts ...optparams.Option[SDKConfig]) error`通过配置初始化sdk实例
6. 使用`func (c *SDK[T]) GetClient(opts ...optparams.Option[AcquireOptions]) (T, ReleaseFunc)`获取客户端对象和客户端回收函数
7. 使用`func (c *SDK[T]) NewCtx(opts ...optparams.Option[CtxOptions]) (ctx context.Context, cancel context.CancelFunc)`构造请求的上下文和上下文取消函数
8. 调用接口`T`规定的方法.
9. 执行上下文取消函数
10. 执行客户端回收函数
11. 执行`func (c *SDK[T]) Close() error`关闭sdk

补充:

+ 在第四步完成后可以使用`sdk.Logger`打印log,这个log是带字段`"module":"grpcsdk"`和`"target_service": desc.ServiceName`的`*slog.Logger`,它的输出配置基于创建时的全局logger,日志等级会跟随后续`github.com/Golang-Tools/loggerhelper/v4`的`Set`方法变化
+ `Init`可以重复调用,重复调用会关闭旧的客户端获取器并按新配置重建,因此可以在运行期切换配置
+ 未配置证书时sdk使用insecure连接;配置了`Ca_Cert_Path`后使用TLS,同时配置`Client_Cert_Path`与`Client_Key_Path`时使用mTLS
+ `Conn_With_Block`开启后建连会阻塞等待连接就绪(默认最长等待10s),对应`NewClientOptions`中的`BlockUntilReady`与`BlockWaitTime`
+ `Client_Pool`开启后使用客户端连接池,池的安全水位为`Client_Pool_Reservations`(池中保持的客户端数量),最大水位为`Client_Pool_Limits`;`GetClient`返回的回收函数会按安全水位把客户端放回池中,超出安全水位的连接会被关闭
+ 直接使用`NewClient`/`NewPool`而没有配置`DialOpts`时默认使用insecure连接,自己设置`DialOpts`时需要自行提供传输凭证
+ `WithRetryPolicy`可以开启grpc内建的重试(写入service config的默认methodConfig),只有幂等的方法才应该配置
+ `WithConnectParams`/`WithIdleTimeoutMS`可以调整重连退避、单次建连最短超时与空闲连接回收行为(空闲回收默认30分钟,传负数禁用)
+ `WithLoadBalancingPolicy`可以指定负载均衡策略,详见下面的`负载均衡与连接池`

## 负载均衡与连接池

grpc推荐用**单个ClientConn配合负载均衡策略**来承载并发:同一个ClientConn内部会维护到多个后端的子连接,请求按策略分布到不同后端,连接管理与空闲回收由grpc自身负责。因此:

+ 多地址场景直接使用`WithQueryAddresses(a1, a2, ...)`(内部自动组成本地负载均衡),或使用`dns:///`地址;
+ 用`WithLoadBalancingPolicy`指定策略,常用值:`pick_first`(单地址默认)、`round_robin`、`least_request`、`weighted_round_robin`;多地址与dns地址不设置时默认`round_robin`,xds地址由xds配置决定;
+ `Client_Pool`(多个ClientConn的连接池)只在需要连接级隔离等特殊场景使用:每个ClientConn都带有独立的resolver/负载均衡/子连接,池化会成倍放大资源占用,一般情况下不必开启。

## 重试策略

`WithRetryPolicy`可以开启grpc内建的重试(通过service config的默认methodConfig下发):

```go
sdk.Init(
    grpcsdk.WithQueryAddresses("localhost:5000"),
    grpcsdk.WithRetryPolicy(&grpcsdk.RetryPolicy{
        MaxAttempts:          3,
        InitialBackoff:       10 * time.Millisecond,
        MaxBackoff:           100 * time.Millisecond,
        BackoffMultiplier:    2,
        RetryableStatusCodes: []string{"UNAVAILABLE"},
    }),
)
```

注意:

+ 重试只应该用于幂等的方法,否则可能造成重复处理;
+ `MaxAttempts`取值范围为[2,5],状态码列表不能为空,不满足时`Init`会报错;
+ grpc-go当前尚未实现对冲策略(hedging),因此本项目不提供对冲配置。

## 使用例子

```go
package main

import (
    "io"
    "os"

    "github.com/Golang-Tools/grpcsdk/v2"
    log "github.com/Golang-Tools/loggerhelper/v4"
    "xxx_pb"
    "google.golang.org/grpc"
    "google.golang.org/grpc/metadata"
)

func main() {
    sdk := grpcsdk.New(xxx_pb.NewTESTGOGRPCSIMPLEClient, &xxx_pb.TESTGOGRPCSIMPLE_ServiceDesc)
    sdk.Logger.Info("setup sdk ok")
    err := sdk.Init(grpcsdk.WithQueryAddresses("localhost:5000"))
    if err != nil {
        sdk.Logger.Error("sdk.Init get error", "err", err.Error())
    }
    defer sdk.Close()
    sdk.Logger.Info("setup sdk init ok")
    Conn, release := sdk.GetClient()
    defer release()
    sdk.Logger.Info("setup sdk GetClient ok")
    sdk.Logger.Info("setup ok")
    //req-res
    ctx, cancel := sdk.NewCtx(sdk.WithRequestMeta(), grpcsdk.WithMeta("a", "1"), grpcsdk.WithMeta("b", "2"))
    defer cancel()

    var header, trailer metadata.MD
    req, err := Conn.Square(ctx, &xxx_pb.Message{Message: 2.0},
        grpc.Header(&header),   // will retrieve header
        grpc.Trailer(&trailer), // will retrieve trailer
    )
    if err != nil {
        sdk.Logger.Error("Square get error", "err", err.Error())
        os.Exit(1)
    }
    log.Info("Square get result", "header", header, "req", req, "trailer", trailer)

    //req-stream
    streamctx, streamcancel := sdk.NewCtx(grpcsdk.UntilEnd())
    defer streamcancel()
    ResStream, err := Conn.RangeSquare(streamctx, &xxx_pb.Message{Message: 4.0})
    if err != nil {
        sdk.Logger.Error("RangeSquare get error", "err", err.Error())
        os.Exit(1)
    }
    for {
        feature, err := ResStream.Recv()
        if err != nil {
            if err == io.EOF {
                break
            } else {
                sdk.Logger.Error("RangeSquare(_) = _", "err", err.Error())
                os.Exit(1)
            }
        }
        sdk.Logger.Info("RangeSquare get res", "res", feature)
    }
}
```