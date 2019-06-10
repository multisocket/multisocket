package sender

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
	ErrBadDestination  = err("bad destination")
	ErrPipeNotFound    = err("pipe not found")
	ErrInvalidSendType = err("invalid send type")
)
