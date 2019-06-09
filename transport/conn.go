package transport

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/webee/multisocket/options"
)

// connection implements the Connection interface on top of net.Conn.
type connection struct {
	transport Transport
	c         net.Conn
	maxrx     uint32

	sync.Mutex
	closed bool
}

func (conn *connection) Transport() Transport {
	return conn.transport
}

// Recv implements the TranPipe Recv method.  The message received is expected
// as a 32-bit size (network byte order) followed by the message itself.
func (conn *connection) Recv() (msg []byte, err error) {
	var sz uint32

	if err = binary.Read(conn.c, binary.BigEndian, &sz); err != nil {
		return
	}

	if conn.maxrx > 0 && sz > conn.maxrx {
		err = ErrMsgTooLong
		return
	}

	msg = make([]byte, sz)
	if _, err = io.ReadFull(conn.c, msg); err != nil {
		return
	}
	return
}

// Send implements the Pipe Send method.  The message is sent as a 32-bit
// size (network byte order) followed by the message itself.
func (conn *connection) Send(msgs ...[]byte) (err error) {
	var (
		buff = net.Buffers{}
		sz   uint32
	)

	// Serialize the length header
	for _, msg := range msgs {
		sz += uint32(len(msg))
	}
	lbyte := make([]byte, 4)
	binary.BigEndian.PutUint32(lbyte, sz)

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
	return fmt.Sprintf("%s://%s", conn.transport.Scheme(), conn.c.LocalAddr().String())
}

func (conn *connection) RemoteAddress() string {
	return fmt.Sprintf("%s://%s", conn.transport.Scheme(), conn.c.RemoteAddr().String())
}

// NewConnection allocates a new Connection using the supplied net.Conn
func NewConnection(transport Transport, c net.Conn, opts options.Options) (Connection, error) {
	conn := &connection{
		transport: transport,
		c:         c,
	}

	if val, ok := opts.GetOption(OptionMaxRecvMsgSize); ok {
		conn.maxrx = OptionMaxRecvMsgSize.Value(val)
	}

	return conn, nil
}
