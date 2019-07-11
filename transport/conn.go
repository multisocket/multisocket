package transport

import (
	"fmt"
	"net"
)

type (
	// connection implements the Connection interface on top of net.Conn.
	connection struct {
		transport Transport
		net.Conn
	}
)

func (conn *connection) Transport() Transport {
	return conn.transport
}

func (conn *connection) Writev(v ...[]byte) (int64, error) {
	return (*net.Buffers)(&v).WriteTo(conn.Conn)
}

func (conn *connection) LocalAddress() string {
	return fmt.Sprintf("%s://%s", conn.transport.Scheme(), conn.Conn.LocalAddr().String())
}

func (conn *connection) RemoteAddress() string {
	return fmt.Sprintf("%s://%s", conn.transport.Scheme(), conn.Conn.RemoteAddr().String())
}

// NewConnection allocates a new Connection using the net.Conn
func NewConnection(transport Transport, nc net.Conn) (Connection, error) {
	conn := &connection{
		transport: transport,
		Conn:      nc,
	}

	return conn, nil
}
