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
	ErrMsgDropped      = err("message dropped")
	ErrBadDestination  = err("bad destination")
	ErrBrokenPath      = err("broken path")
	ErrInvalidSendType = err("invalid send type")
)
