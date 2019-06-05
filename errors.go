package multisocket

type err string

func (e err) Error() string {
	return string(e)
}

// Predefined error values.
const (
	ErrBadAddr     = err("invalid address")
	ErrBadHeader   = err("invalid header received")
	ErrBadVersion  = err("invalid protocol version")
	ErrTooShort    = err("message is too short")
	ErrTooLong     = err("message is too long")
	ErrClosed      = err("object closed")
	ErrConnRefused = err("connection refused")
	ErrSendTimeout = err("send time out")
	ErrRecvTimeout = err("receive time out")
	ErrProtoState  = err("incorrect protocol state")
	ErrProtoOp     = err("invalid operation for protocol")
	ErrBadTran     = err("invalid or unsupported transport")
	ErrBadProto    = err("invalid or unsupported protocol")
	ErrBadOption   = err("invalid or unsupported option")
	ErrBadValue    = err("invalid option value")
	ErrGarbled     = err("message garbled")
	ErrAddrInUse   = err("address in use")
	ErrBadProperty = err("invalid property name")
	ErrTLSNoConfig = err("missing TLS configuration")
	ErrTLSNoCert   = err("missing TLS certificates")
	ErrNotRaw      = err("socket not raw")
	ErrCanceled    = err("operation canceled")
	ErrNoContext   = err("protocol does not support contexts")
)