# grpcsdk

grpc的客户端sdk模板,使用它快速构造grpc的sdk

本项目只适用于go 1.18+

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

+ 在第四步完成后可以使用`sdk.Logger`打印log,这个log会带有字段`"module":"grpcsdk"`和`"target_service": desc.ServiceName`,log的其它配置会根据`github.com/Golang-Tools/loggerhelper/v2`的`Set`方法变化而变化

## 使用例子

```go
package main

import (
    "io"
    "os"

    "github.com/Golang-Tools/grpcsdk"
    "xxx_pb"
    log "github.com/Golang-Tools/loggerhelper/v2"
    "google.golang.org/grpc"
    "google.golang.org/grpc/metadata"
)

func main() {
    sdk := grpcsdk.New(xxx_pb.NewTESTGOGRPCSIMPLEClient, &xxx_pb.TESTGOGRPCSIMPLE_ServiceDesc)
    sdk.Logger.Info("setup sdk ok")
    err := sdk.Init(grpcsdk.WithQueryAddresses("localhost:5000"))
    if err != nil {
        sdk.Logger.Error("sdk.Init get error", log.Dict{"err": err.Error()})
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
        sdk.Logger.Error("Square get error", log.Dict{"err": err.Error()})
        os.Exit(1)
    }
    log.Info("Square get result", log.Dict{"header": header, "req": req, "trailer": trailer})

    //req-stream
    streamctx, streamcancel := sdk.NewCtx(grpcsdk.UntilEnd())
    defer streamcancel()
    ResStream, err := Conn.RangeSquare(streamctx, &xxx_pb.Message{Message: 4.0})
    if err != nil {
        sdk.Logger.Error("RangeSquare get error", log.Dict{"err": err.Error()})
        os.Exit(1)
    }
    for {
        feature, err := ResStream.Recv()
        if err != nil {
            if err == io.EOF {
                break
            } else {
                sdk.Logger.Error("RangeSquare(_) = _", log.Dict{"err": err.Error()})
                os.Exit(1)
            }
        }
        sdk.Logger.Info("RangeSquare get res", log.Dict{"res": feature})
    }
}
```