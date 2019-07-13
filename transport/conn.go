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
		laddr string
		raddr string
	}
)

func (conn *connection) Transport() Transport {
	return conn.transport
}

func (conn *connection) Writev(v ...[]byte) (int64, error) {
	return (*net.Buffers)(&v).WriteTo(conn.Conn)
}

func (conn *connection) LocalAddress() string {
	return conn.laddr
}

func (conn *connection) RemoteAddress() string {
	return conn.raddr
}

// NewConnection allocates a new Connection using the net.Conn
func NewConnection(transport Transport, nc net.Conn, accepted bool) (Connection, error) {
	conn := &connection{
		transport: transport,
		Conn:      nc,
	}
	if accepted {
		conn.laddr = fmt.Sprintf("%s://%s", conn.transport.Scheme(), conn.Conn.LocalAddr().String())
		conn.raddr = fmt.Sprintf("%s://%s", conn.Conn.RemoteAddr().Network(), conn.Conn.RemoteAddr().String())
	} else {
		conn.laddr = fmt.Sprintf("%s://%s", conn.Conn.LocalAddr().Network(), conn.Conn.LocalAddr().String())
		conn.raddr = fmt.Sprintf("%s://%s", conn.transport.Scheme(), conn.Conn.RemoteAddr().String())
	}

	return conn, nil
}
