package netpipe

import (
	"net"

	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/transport"
	"github.com/multisocket/multisocket/transport/inproc"
)

var (
	// Transport is inproc transport based on net.Pipe
	Transport = inproc.NewTransport("inproc.netpipe", newPipe)
)

func init() {
	transport.RegisterTransport(Transport)
}

func newPipe(opts options.Options) (net.Conn, net.Conn) {
	return net.Pipe()
}