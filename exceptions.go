package grpcsdk

import "errors"

var (
	// ErrClosed is the error when the client pool is closed
	ErrClosed = errors.New("grpc pool: client pool is closed")
	// ErrTimeout is the error when the client pool timed out
	ErrTimeout = errors.New("grpc pool: client pool timed out")
	// ErrAlreadyClosed is the error when the client conn was already closed
	ErrAlreadyClosed = errors.New("grpc pool: the connection was already closed")
	// ErrFullPool is the error when the pool is already full
	ErrFullPool = errors.New("grpc pool: closing a ClientConn into a full pool")
	//ErrLimitsSmallThanReservation 初始化参数limits比reservations小
	ErrLimitsSmallThanReservation = errors.New("init param limits must larger than reservations")
	//ErrReservationSmallThanOne 初始化参数reservations比1小
	ErrReservationSmallThanOne = errors.New("init param reservation must larger than 1")
)
