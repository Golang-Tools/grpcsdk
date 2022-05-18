package grpcsdk

import (
	"github.com/Golang-Tools/optparams"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

//GrpcConnInterface grpc的连接接口
type GrpcConnInterface interface {
	grpc.ClientConnInterface
	GetState() connectivity.State
	Connect()
	ResetConnectBackoff()
	Close() error
}

//GrpcClientInterface grpc的客户端接口
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
type GrpcClientInterface[T any] interface {
	GetConn() GrpcConnInterface
	AsGrpcClient() T
	Close() error
}

//GrpcClientGetter 客户获取客户端的对象
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
type GrpcClientGetter[T any] interface {
	Acquire(...optparams.Option[AcquireOptions]) (GrpcClientInterface[T], error)
	Release(cli GrpcClientInterface[T])
	Close() error
}

//NewGrpcClientFunc 用于构造可以被池使用对象的工厂函数
//@generics T any 由pb生成的客户端接口,以`XXXXClient`命名的interface
type NewGrpcClientFunc[T any] func(grpc.ClientConnInterface) T

type ReleaseFunc func()
