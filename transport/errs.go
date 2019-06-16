package transport

type err string

func (e err) Error() string {
	return string(e)
}

// errors
const (
	ErrConnRefused  = err("connection refused")
	ErrNotListening = err("not listening")
)
