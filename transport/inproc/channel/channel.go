package channel

import (
	"net"
	"sync"
	"time"

	"github.com/multisocket/multisocket/bytespool"
	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/message"
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

	msrPipe struct {
		*pipe
		lastMsg *message.Message
		recvq   <-chan *message.Message
		sendq   chan<- *message.Message
	}

	srPipe struct {
		*pipe
		lastBytes []byte
	}
)

var (
	// RwTransport is inproc transport based on bytes channel, using ReadWriter
	RwTransport = inproc.NewTransport("inproc.channel.rw", newPipe)
	// MsrTransport is inproc transport based on bytes channel, using MsgSendReceiver+ReadWriter
	MsrTransport = inproc.NewTransport("inproc.channel.msr", newMsrPipe)
	// SrTransport is inproc transport based on bytes channel, using SendReceiver+ReadWriter
	SrTransport = inproc.NewTransport("inproc.channel.sr", newSrPipe)
)

func init() {
	transport.RegisterTransport(RwTransport)
	transport.RegisterTransport(MsrTransport)
	transport.RegisterTransport(SrTransport)
	// default inproc.channel
	transport.RegisterTransportWithScheme(MsrTransport, "inproc.channel")
}

func newPipe(laddr, raddr net.Addr, opts options.Options) (net.Conn, net.Conn) {
	return createPipe(laddr, raddr, opts)
}

func newMsrPipe(laddr, raddr net.Addr, opts options.Options) (net.Conn, net.Conn) {
	bufferSize := opts.GetOptionDefault(Options.BufferSize).(int)
	ma, mb := make(chan *message.Message, bufferSize), make(chan *message.Message, bufferSize)
	pa, pb := &msrPipe{
		sendq: ma,
		recvq: mb,
	}, &msrPipe{
		sendq: mb,
		recvq: ma,
	}
	pa.pipe, pb.pipe = createPipe(laddr, raddr, opts)

	return pa, pb
}

func newSrPipe(laddr, raddr net.Addr, opts options.Options) (net.Conn, net.Conn) {
	pa, pb := &srPipe{}, &srPipe{}
	pa.pipe, pb.pipe = createPipe(laddr, raddr, opts)

	return pa, pb
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

// MsgSendReceiver

func (p *msrPipe) SendMsg(msg *message.Message) (err error) {
	select {
	case <-p.closedq:
		err = errs.ErrClosed
	case p.sendq <- msg:
	}
	return
}

func (p *msrPipe) RecvMsg() (msg *message.Message, err error) {
	if p.lastMsg != nil {
		p.lastMsg.FreeAll()
		p.lastMsg = nil
	}
	select {
	case <-p.closedq:
		// exhaust received messages
		select {
		case msg = <-p.recvq:
		default:
			err = errs.ErrClosed
		}
	case msg = <-p.recvq:
	}
	p.lastMsg = msg
	return
}

// SendReceiver

func (p *srPipe) Send(b []byte) (err error) {
	select {
	case <-p.closedq:
		err = errs.ErrClosed
	case p.wc <- b:
	}
	return
}

func (p *srPipe) Recv() (b []byte, err error) {
	if p.lastBytes != nil {
		bytespool.Free(p.lastBytes)
		p.lastBytes = nil
	}
	select {
	case <-p.closedq:
		// exhaust received messages
		select {
		case b = <-p.rc:
		default:
			err = errs.ErrClosed
		}
	case b = <-p.rc:
	}
	p.lastBytes = b
	return
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
