package transport

import (
	"github.com/multisocket/multisocket/errs"
)

// errors
const (
	ErrConnRefused  = errs.Err("connection refused")
	ErrNotListening = errs.Err("not listening")
)
