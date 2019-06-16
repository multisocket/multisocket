package errs

type err string

func (e err) Error() string {
	return string(e)
}

// errors
const (
	ErrClosed                = err("object is closed")
	ErrTimeout               = err("operation time out")
	ErrAddrInUse             = err("address in use")
	ErrOperationNotSupported = err("operation not supported")
	ErrBadTransport          = err("invalid or unsupported transport")
	ErrBadMsg                = err("bad message")
	ErrMsgTooLong            = err("message is too long")
)
