package channel

import (
	"net"

	"github.com/multisocket/multisocket/bytespool"
	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/transport"
	"github.com/multisocket/multisocket/transport/inproc"
)

type (
	srPipe struct {
		*pipe
		lastBytes []byte
	}
)

var (
	// SrTransport is inproc transport based on bytes channel, using SendReceiver+ReadWriter
	SrTransport = inproc.NewTransport("inproc.channel.sr", newSrPipe)
)

func init() {
	transport.RegisterTransport(SrTransport)
}

func newSrPipe(laddr, raddr net.Addr, opts options.Options) (net.Conn, net.Conn) {
	pa, pb := &srPipe{}, &srPipe{}
	pa.pipe, pb.pipe = createPipe(laddr, raddr, opts)

	return pa, pb
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
