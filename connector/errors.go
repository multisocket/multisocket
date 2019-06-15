package connector

import (
	"github.com/webee/multisocket/errors"
)

type err string

func (e err) Error() string {
	return string(e)
}

// errors
const (
	ErrClosed    = errors.ErrClosed
	ErrTimeout   = errors.ErrTimeout
	ErrAddrInUse = errors.ErrAddrInUse
)
