package transport

import (
	"fmt"
	"net"
)

type (
	// connection implements Transport.Connection interface
	connection struct {
		tran Transport
		net.Conn
		laddr string
		raddr string
	}
)

func (conn *connection) Transport() Transport {
	return conn.tran
}

// Writev write multiple bytes at once
func (conn *connection) Writev(v ...[]byte) (int64, error) {
	return (*net.Buffers)(&v).WriteTo(conn.Conn)
}

// LocalAddress return local address
func (conn *connection) LocalAddress() string {
	return conn.laddr
}

// RemoteAddress return remote address
func (conn *connection) RemoteAddress() string {
	return conn.raddr
}

func (conn *connection) RawConn() net.Conn {
	return conn.Conn
}

// NewConnection allocates a new Connection using the net.Conn
func NewConnection(transport Transport, nc net.Conn, accepted bool) (Connection, error) {
	conn := &connection{
		tran: transport,
		Conn: nc,
	}
	if accepted {
		conn.laddr = fmt.Sprintf("%s://%s", conn.tran.Scheme(), conn.Conn.LocalAddr().String())
		conn.raddr = fmt.Sprintf("%s://%s", conn.Conn.RemoteAddr().Network(), conn.Conn.RemoteAddr().String())
	} else {
		conn.laddr = fmt.Sprintf("%s://%s", conn.Conn.LocalAddr().Network(), conn.Conn.LocalAddr().String())
		conn.raddr = fmt.Sprintf("%s://%s", conn.tran.Scheme(), conn.Conn.RemoteAddr().String())
	}

	return conn, nil
}
