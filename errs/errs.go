package errs

// Err is the error type
type Err string

func (e Err) Error() string {
	return string(e)
}

// errors
const (
	ErrClosed                = Err("object is closed")
	ErrTimeout               = Err("operation time out")
	ErrBadOperateState       = Err("bad operation state")
	ErrBadAddr               = Err("bad address")
	ErrAddrInUse             = Err("address already in use")
	ErrOperationNotSupported = Err("operation not supported")
	ErrBadTransport          = Err("invalid or unsupported transport")
	ErrBadMsg                = Err("bad message")
	ErrBadProtocol           = Err("bad protocol")
	ErrContentTooLong        = Err("content is too long")
)
