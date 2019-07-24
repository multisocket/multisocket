package netpipe

import (
	"net"

	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/transport"
	"github.com/multisocket/multisocket/transport/inproc"
)

type (
	pipe struct {
		net.Conn
		laddr net.Addr
		raddr net.Addr
	}
)

var (
	// Transport is inproc transport based on net.Pipe
	Transport = inproc.NewTransport("inproc.netpipe", newPipe)
)

func init() {
	transport.RegisterTransport(Transport)
}

func newPipe(laddr, raddr net.Addr, opts options.Options) (net.Conn, net.Conn) {
	lc, rc := net.Pipe()

	return &pipe{
			Conn:  lc,
			laddr: laddr,
			raddr: raddr,
		}, &pipe{
			Conn:  rc,
			laddr: raddr,
			raddr: laddr,
		}
}

func (p *pipe) LocalAddr() net.Addr {
	return p.laddr
}

func (p *pipe) RemoteAddr() net.Addr {
	return p.raddr
}
