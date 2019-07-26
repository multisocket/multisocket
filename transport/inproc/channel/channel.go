package channel

import (
	"net"
	"sync"
	"time"

	"github.com/multisocket/multisocket/bytespool"
	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/transport"
	"github.com/multisocket/multisocket/transport/inproc"
)

type (
	pipe struct {
		rc      <-chan []byte
		s       []byte
		i       int64 // current reading index
		wc      chan<- []byte
		lk      *sync.Mutex
		closedq chan struct{}

		laddr net.Addr
		raddr net.Addr
	}
)

var (
	// RwTransport is inproc transport based on bytes channel, using ReadWriter
	RwTransport = inproc.NewTransport("inproc.channel.rw", newPipe)
)

func init() {
	transport.RegisterTransport(RwTransport)
}

func newPipe(laddr, raddr net.Addr, opts options.Options) (net.Conn, net.Conn) {
	return createPipe(laddr, raddr, opts)
}

func createPipe(laddr, raddr net.Addr, opts options.Options) (*pipe, *pipe) {
	bufferSize := opts.GetOptionDefault(Options.BufferSize).(int)
	a, b := make(chan []byte, bufferSize), make(chan []byte, bufferSize)
	lk := &sync.Mutex{}
	closedq := make(chan struct{})
	return &pipe{
			rc:      a,
			wc:      b,
			lk:      lk,
			closedq: closedq,

			laddr: laddr,
			raddr: raddr,
		}, &pipe{
			rc:      b,
			wc:      a,
			lk:      lk,
			closedq: closedq,

			laddr: raddr,
			raddr: laddr,
		}
}

// pipe

func (p *pipe) Close() error {
	p.lk.Lock()
	defer p.lk.Unlock()
	select {
	case <-p.closedq:
		return errs.ErrClosed
	default:
		close(p.closedq)
	}

	return nil
}

// ReadWriter

func (p *pipe) Read(b []byte) (n int, err error) {
READING:
	for {
		if p.s == nil {
			select {
			case <-p.closedq:
				// read remaining
				select {
				case p.s = <-p.rc:
				default:
					err = errs.ErrClosed
					return
				}
			case p.s = <-p.rc:
			}
			p.i = 0
		}
		// reading
		if p.i >= int64(len(p.s)) {
			bytespool.Free(p.s)
			p.s = nil
			continue READING
		} else {
			n = copy(b, p.s[p.i:])
			p.i += int64(n)
		}
		return
	}
}

func (p *pipe) Write(b []byte) (n int, err error) {
	select {
	case <-p.closedq:
		err = errs.ErrClosed
		return
	case p.wc <- b:
		n = len(b)
	}
	return
}

func (p *pipe) LocalAddr() net.Addr {
	return p.laddr
}

func (p *pipe) RemoteAddr() net.Addr {
	return p.raddr
}

func (p *pipe) SetDeadline(t time.Time) error {
	// TODO: add read/write timeout
	return errs.ErrOperationNotSupported
}

func (p *pipe) SetReadDeadline(t time.Time) error {
	return errs.ErrOperationNotSupported
}

func (p *pipe) SetWriteDeadline(t time.Time) error {
	return errs.ErrOperationNotSupported
}
