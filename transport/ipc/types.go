package ipc

import (
	"github.com/webee/multisocket/transport"
)

type (
	ipcTran string
)

const (
	// Transport is a transport.Transport for IPC.
	Transport = ipcTran("ipc")
)

func init() {
	transport.RegisterTransport(Transport)
}

// Scheme implements the Transport Scheme method.
func (t ipcTran) Scheme() string {
	return string(t)
}
