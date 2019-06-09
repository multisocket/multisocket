package transport

import (
	"github.com/webee/multisocket"
)

type err string

func (e err) Error() string {
	return string(e)
}

// errors
const (
	ErrClosed     = multisocket.ErrClosed
	ErrOperationNotSupported = multisocket.ErrOperationNotSupported
	ErrBadTran    = err("invalid or unsupported transport")
	ErrMsgTooLong = err("message is too long")
)
