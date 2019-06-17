package ipc

import (
	"github.com/webee/multisocket/transport"
)

type (
	ipcTran int
)

const (
	// Transport is a transport.Transport for IPC.
	Transport = ipcTran(0)
	scheme    = "ipc"
)

func init() {
	transport.RegisterTransport(Transport)
}

// Scheme implements the Transport Scheme method.
func (ipcTran) Scheme() string {
	return scheme
}
