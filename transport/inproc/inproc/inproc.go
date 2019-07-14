package inproc

import (
	"github.com/webee/multisocket/transport"
	"github.com/webee/multisocket/transport/inproc/channel"
)

func init() {
	// as default inproc transport
	transport.RegisterTransportWithScheme(channel.Transport, "inproc")
}
