package sender

type err string

func (e err) Error() string {
	return string(e)
}

// sender errors
const (
	ErrPipeClosed = err("pipe is closed")
)
