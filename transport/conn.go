package transport

import (
	"fmt"
	"net"
)

type (
	// connection implements the Connection interface on top of net.Conn.
	connection struct {
		transport Transport
		PrimitiveConnection
	}

	// PrimitiveConnection is primitive connection made by transport.
	PrimitiveConnection interface {
		Read(b []byte) (n int, err error)
		Write(b []byte) (n int, err error)
		Close() error
		LocalAddress() string
		RemoteAddress() string
	}

	primitiveConn struct {
		net.Conn
	}
)

func (pc *primitiveConn) LocalAddress() string {
	return pc.Conn.LocalAddr().String()
}

func (pc *primitiveConn) RemoteAddress() string {
	return pc.Conn.RemoteAddr().String()
}

func (conn *connection) Transport() Transport {
	return conn.transport
}

func (conn *connection) LocalAddress() string {
	return fmt.Sprintf("%s://%s", conn.transport.Scheme(), conn.PrimitiveConnection.LocalAddress())
}

func (conn *connection) RemoteAddress() string {
	return fmt.Sprintf("%s://%s", conn.transport.Scheme(), conn.PrimitiveConnection.RemoteAddress())
}

// NewPrimitiveConn convert net.Conn to PrimitiveConnection
func NewPrimitiveConn(c net.Conn) PrimitiveConnection {
	return &primitiveConn{c}
}

// NewConnection allocates a new Connection using the PrimitiveConnection
func NewConnection(transport Transport, pc PrimitiveConnection) (Connection, error) {
	conn := &connection{
		transport:           transport,
		PrimitiveConnection: pc,
	}

	return conn, nil
}
