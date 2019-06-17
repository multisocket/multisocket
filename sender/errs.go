package sender

import (
	"github.com/webee/multisocket/errs"
)

// errors
const (
	ErrMsgDropped      = errs.Err("message dropped")
	ErrBadDestination  = errs.Err("bad destination")
	ErrBrokenPath      = errs.Err("broken path")
	ErrInvalidSendType = errs.Err("invalid send type")
)
