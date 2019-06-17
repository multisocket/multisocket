package errs

type Err string

func (e Err) Error() string {
	return string(e)
}

// errors
const (
	ErrClosed                = Err("object is closed")
	ErrTimeout               = Err("operation time out")
	ErrAddrInUse             = Err("address in use")
	ErrOperationNotSupported = Err("operation not supported")
	ErrBadTransport          = Err("invalid or unsupported transport")
	ErrBadMsg                = Err("bad message")
	ErrMsgTooLong            = Err("message is too long")
)
