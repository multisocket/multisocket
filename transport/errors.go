package transport

import (
	"github.com/webee/multisocket/errors"
)

type err string

func (e err) Error() string {
	return string(e)
}

// errors
const (
	ErrClosed                = errors.ErrClosed
	ErrOperationNotSupported = errors.ErrOperationNotSupported
	ErrBadTransport          = errors.ErrBadTransport
	ErrBadMsg                = errors.ErrBadMsg
	ErrMsgTooLong            = errors.ErrMsgTooLong
	ErrConnRefused           = err("connection refused")
	ErrNotListening          = err("not listening")
)
