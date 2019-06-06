package receiver

type err string

func (e err) Error() string {
	return string(e)
}

// sender errors
const (
	ErrPipeClosed    = err("pipe is closed")
	ErrRecvNotAllowd = err("recv is not allowd")
)
