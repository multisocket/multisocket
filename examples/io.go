package examples

import (
	"bytes"
	"errors"
	"io"
	"sync"
)

type (
	// MultiplexReader let a reader be read by multiple consumer.
	MultiplexReader struct {
		sync.Mutex
		closed bool
		r      io.Reader
		bufs   map[*xbuf]struct{}
	}

	xbuf struct {
		sync.Mutex
		closedq chan struct{}
		mr      *MultiplexReader
		ready   chan struct{}
		buf     *bytes.Buffer
	}
)

var (
	// ErrClosed is the error used for operation on a closed object.
	ErrClosed = errors.New("object closed")
)

// NewMultiplexReader create a MultiplexReader
func NewMultiplexReader(r io.Reader) *MultiplexReader {
	mr := &MultiplexReader{
		r:    r,
		bufs: make(map[*xbuf]struct{}),
	}
	go mr.run()
	return mr
}

func (mr *MultiplexReader) run() {
	var (
		n   int
		err error
	)
	p := make([]byte, 32*1024)
	for {
		n, err = mr.r.Read(p)
		if n > 0 {
			bufs := mr.getBufs()
			for _, buf := range bufs {
				n, err = buf.buf.Write(p[:n])
				// ready signal, best effort
				select {
				case buf.ready <- struct{}{}:
				default:
				}
				if err != nil {
					buf.Close()
				}
			}
		}
		if err != nil {
			break
		}
	}
	mr.Close()
}

func (mr *MultiplexReader) getBufs() []*xbuf {
	bufs := make([]*xbuf, len(mr.bufs))
	mr.Lock()
	idx := 0
	for buf := range mr.bufs {
		bufs[idx] = buf
		idx++
	}
	mr.Unlock()
	return bufs
}

// Close close all readers.
func (mr *MultiplexReader) Close() error {
	mr.Lock()
	if mr.closed {
		mr.Unlock()
		return io.EOF
	}
	mr.closed = true
	mr.Unlock()

	bufs := mr.getBufs()
	for _, buf := range bufs {
		buf.Close()
	}
	return nil
}

// NewReader create a new reader from source
func (mr *MultiplexReader) NewReader() (io.ReadCloser, error) {
	mr.Lock()
	if mr.closed {
		mr.Unlock()
		return nil, ErrClosed
	}
	mr.Unlock()

	buf := &xbuf{
		mr:      mr,
		closedq: make(chan struct{}),
		ready:   make(chan struct{}, 1),
		buf:     bytes.NewBuffer(nil),
	}
	mr.Lock()
	mr.bufs[buf] = struct{}{}
	mr.Unlock()
	return buf, nil
}

func (b *xbuf) Read(p []byte) (n int, err error) {
	select {
	case <-b.closedq:
		err = io.EOF
		return
	case <-b.ready:
		n, err = b.buf.Read(p)
		if err == nil {
			// err is nil means buf may has more data to read.
			select {
			case b.ready <- struct{}{}:
			default:
			}
		}
		err = nil
	}
	return
}

func (b *xbuf) Close() error {
	b.Lock()
	select {
	case <-b.closedq:
		b.Unlock()
		return ErrClosed
	default:
		close(b.closedq)
	}
	b.Unlock()

	b.mr.Lock()
	delete(b.mr.bufs, b)
	b.mr.Unlock()
	return nil
}
