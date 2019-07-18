package multisocket

import (
	"github.com/multisocket/multisocket/errs"
)

// errors
const (
	ErrMsgDropped      = errs.Err("message dropped")
	ErrBrokenPath      = errs.Err("bad destination: broken path")
	ErrInvalidSendType = errs.Err("invalid send type")
)
