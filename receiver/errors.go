package receiver

import (
	"github.com/webee/multisocket"
)

type err string

func (e err) Error() string {
	return string(e)
}

// errors
const (
	ErrClosed          = multisocket.ErrClosed
	ErrTimeout         = multisocket.ErrTimeout
	ErrRecvInvalidData = err("recv invalid data")
	ErrRecvNotAllowd   = err("recv is not allowd")
)
