package connector

type err string

func (e err) Error() string {
	return string(e)
}

// errors
const (
	ErrStopped = err("object is stopped")
)
