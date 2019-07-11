package netpipe

import (
	"net"

	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/transport"
	"github.com/webee/multisocket/transport/inproc"
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