package errs

type Err string

func (e Err) Error() string {
	return string(e)
}

// errors
const (
	ErrClosed                = Err("object is closed")
	ErrTimeout               = Err("operation time out")
	ErrBadOperateState       = Err("bad operation state")
	ErrAddrInUse             = Err("address already in use")
	ErrOperationNotSupported = Err("operation not supported")
	ErrBadTransport          = Err("invalid or unsupported transport")
	ErrBadMsg                = Err("bad message")
	ErrMsgTooLong            = Err("message is too long")
)
