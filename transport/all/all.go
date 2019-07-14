// Package all is used to register all transports.  This allows a program
// to support all known transports as well as supporting as yet-unknown
// transports, with a single import.
package all

import (
	// import transports
	_ "github.com/multisocket/multisocket/transport/inproc/inproc"
	_ "github.com/multisocket/multisocket/transport/inproc/channel"
	_ "github.com/multisocket/multisocket/transport/inproc/iopipe"
	_ "github.com/multisocket/multisocket/transport/inproc/netpipe"
	_ "github.com/multisocket/multisocket/transport/ipc"
	_ "github.com/multisocket/multisocket/transport/tcp"
	_ "github.com/multisocket/multisocket/transport/ws"
)
