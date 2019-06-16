package receiver

import (
	"github.com/webee/multisocket/errs"
)

type err string

func (e err) Error() string {
	return string(e)
}

// errors
const (
	ErrClosed                = errs.ErrClosed
	ErrTimeout               = errs.ErrTimeout
	ErrOperationNotSupported = errs.ErrOperationNotSupported
	ErrRecvInvalidData       = err("recv invalid data")
)
