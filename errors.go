package multisocket

type err string

func (e err) Error() string {
	return string(e)
}

// errors
const (
	ErrClosed = err("object is closed")
)
