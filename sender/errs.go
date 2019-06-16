package sender

import (
	"github.com/webee/multisocket/errs"
)

type err string

func (e err) Error() string {
	return string(e)
}

// errors
const (
	ErrClosed          = errs.ErrClosed
	ErrTimeout         = errs.ErrTimeout
	ErrMsgDropped      = err("message dropped")
	ErrBadDestination  = err("bad destination")
	ErrBrokenPath      = err("broken path")
	ErrInvalidSendType = err("invalid send type")
)
