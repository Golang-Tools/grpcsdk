package grpcsdk

import "errors"

var (
	//ErrClosed 池已经关闭,无法再获取或释放连接
	ErrClosed = errors.New("grpc pool: client pool is closed")
	//ErrTimeout 从池中获取连接超时
	ErrTimeout = errors.New("grpc pool: client pool timed out")
	//ErrAlreadyClosed 连接已经关闭
	ErrAlreadyClosed = errors.New("grpc pool: the connection was already closed")
	//ErrFullPool 池已经满了
	ErrFullPool = errors.New("grpc pool: closing a ClientConn into a full pool")
	//ErrLimitsSmallThanReservation 初始化参数limits比reservations小
	ErrLimitsSmallThanReservation = errors.New("grpc pool: init param limits must be no less than reservations")
	//ErrReservationSmallThanOne 初始化参数reservations比1小
	ErrReservationSmallThanOne = errors.New("grpc pool: init param reservation must be no less than 1")

	//ErrNoAddresses SDK初始化时没有配置任何地址
	ErrNoAddresses = errors.New("grpcsdk: at least one address is required")
	//ErrEmptyAddr 客户端的连接地址为空
	ErrEmptyAddr = errors.New("grpcsdk: target address is empty")
	//ErrNilClientFactory 没有提供由pb生成的客户端构造函数
	ErrNilClientFactory = errors.New("grpcsdk: grpc client factory is nil")
	//ErrCACertNotParsed 根证书文件中没有解析出有效的证书
	ErrCACertNotParsed = errors.New("grpcsdk: no valid certificate parsed from ca file")
)
