package inproc

import (
	"github.com/multisocket/multisocket/transport"
	"github.com/multisocket/multisocket/transport/inproc/channel"
)

func init() {
	// as default inproc transport
	transport.RegisterTransportWithScheme(channel.Transport, "inproc")
}
