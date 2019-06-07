package connector

import (
	"github.com/webee/multisocket"
)

type err string

func (e err) Error() string {
	return string(e)
}

// errors
const (
	ErrAddrInUse = err("address in use")
	ErrClosed    = multisocket.ErrClosed
)
