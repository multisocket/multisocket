package stream

import (
	"io"
)

var (
	nilQ    chan struct{}
	closedQ = make(chan struct{})
)

func init() {
	close(closedQ)
}

// NewProxy create a stream proxy
func NewProxy() (backStream Stream, frontStream Stream) {
	backStream = New()
	frontStream = New()

	go accept(frontStream, backStream)
	return
}

func accept(frontStream, backStream Stream) {
	var (
		err       error
		frontConn Connection
		backConnq = make(chan Connection, 8)
	)

	// accept back stream
	go func() {
		for {
			if conn, err := backStream.Accept(); err != nil {
				break
			} else {
				backConnq <- conn
			}
		}
	}()

	// accept front stream
	for {
		if frontConn, err = frontStream.Accept(); err != nil {
			break
		}

		go match(frontConn, backStream, backConnq)
	}
}

func match(frontConn Connection, backStream Stream, backConnq chan Connection) {
	cq := closedQ

	var backConn Connection
MATCHING:
	for {
		select {
		case backConn = <-backConnq:
		default:
			select {
			case backConn = <-backConnq:
			case <-cq:
				go func() {
					if conn, err := backStream.Connect(0); err == nil {
						backConnq <- conn
					}
					cq = closedQ
				}()
				cq = nilQ
				continue MATCHING
			}
		}

		if backConn.Closed() {
			continue
		}
		if frontConn.Closed() {
			backConnq <- backConn
			break MATCHING
		}

		go forward(frontConn, backConn)
		go forward(backConn, frontConn)
		break MATCHING
	}
}

func forward(fromConn, toConn Connection) {
	io.Copy(toConn, fromConn)
	toConn.Close()
	fromConn.Close()
}
