package transport

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/webee/multisocket"
)

// connection implements the Connection interface on top of net.Conn.
type connection struct {
	multisocket.Options

	scheme string
	c      net.Conn
	maxrx  int

	sync.Mutex
	closed bool
}

// Recv implements the TranPipe Recv method.  The message received is expected
// as a 64-bit size (network byte order) followed by the message itself.
func (conn *connection) Recv() ([]byte, error) {

	var sz int64
	var err error
	var msg []byte

	if err = binary.Read(conn.c, binary.BigEndian, &sz); err != nil {
		return nil, err
	}

	if sz < 0 || (conn.maxrx > 0 && sz > int64(conn.maxrx)) {
		return nil, multisocket.ErrTooLong
	}

	msg = make([]byte, sz)
	if _, err = io.ReadFull(conn.c, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// Send implements the Pipe Send method.  The message is sent as a 64-bit
// size (network byte order) followed by the message itself.
func (conn *connection) Send(msgs ...[]byte) error {
	var buff = net.Buffers{}

	// Serialize the length header
	l := uint64(0)
	for _, msg := range msgs {
		l += uint64(len(msg))
	}
	lbyte := make([]byte, 8)
	binary.BigEndian.PutUint64(lbyte, l)

	// Attach the length header along with the actual header and body
	buff = append(buff, lbyte)
	buff = append(buff, msgs...)

	if _, err := buff.WriteTo(conn.c); err != nil {
		return err
	}

	return nil
}

// Close implements the Pipe Close method.
func (conn *connection) Close() error {
	conn.Lock()
	defer conn.Unlock()
	if conn.closed {
		return nil
	}
	conn.closed = true

	return conn.c.Close()
}

func (conn *connection) LocalAddress() string {
	return fmt.Sprintf("%s://%s", conn.scheme, conn.c.LocalAddr().String())
}

func (conn *connection) RemoteAddress() string {
	return fmt.Sprintf("%s://%s", conn.scheme, conn.c.RemoteAddr().String())
}

// NewConnection allocates a new Connection using the supplied net.Conn
func NewConnection(scheme string, c net.Conn, opts multisocket.Options) (Connection, error) {
	conn := &connection{
		scheme: scheme,
		c:      c,
		maxrx:  0,
	}

	if val, ok := opts.GetOption(OptionMaxRecvSize); ok {
		conn.maxrx = OptionMaxRecvSize.Value(val)
	}

	return conn, nil
}
