package inproc

import (
	"github.com/webee/multisocket/transport"
	"github.com/webee/multisocket/transport/inproc/iopipe"
)

func init() {
	// as default inproc transport
	transport.RegisterTransportWithScheme(iopipe.Transport, "inproc")
}
