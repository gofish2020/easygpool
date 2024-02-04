package easygpool

import "errors"

var (
	ErrPoolClosed   = errors.New("pool is closed")
	ErrPoolOverload = errors.New("pool is overload")
	ErrTimeout      = errors.New("operation timed out")
)
