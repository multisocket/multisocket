package transport

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/webee/multisocket/errs"
	"github.com/webee/multisocket/options"
)

type (
	// connection implements the Connection interface on top of net.Conn.
	connection struct {
		transport Transport
		raw       bool
		c         PrimitiveConnection
		maxrx     uint32

		sync.Mutex
		closed bool
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

func (conn *connection) IsRaw() bool {
	return conn.raw
}

// Recv implements the TranPipe Recv method.  The message received is expected
// as a 32-bit size (network byte order) followed by the message itself.
func (conn *connection) Recv() (msg []byte, err error) {
	var sz uint32

	if err = binary.Read(conn.c, binary.BigEndian, &sz); err != nil {
		err = errs.ErrBadMsg
		return
	}

	if conn.maxrx > 0 && sz > conn.maxrx {
		err = errs.ErrMsgTooLong
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
func (conn *connection) Send(msg []byte, extras ...[]byte) (err error) {
	var (
		buff = net.Buffers{}
		sz   uint32
	)

	sz = uint32(len(msg))
	// Serialize the length header
	for _, m := range extras {
		sz += uint32(len(m))
	}
	lbyte := make([]byte, 4)
	binary.BigEndian.PutUint32(lbyte, sz)

	// Attach the length header along with the actual header and body
	buff = append(buff, lbyte)
	if len(msg) > 0 {
		buff = append(buff, msg)
	}
	for _, ex := range extras {
		if len(ex) > 0 {
			buff = append(buff, ex)
		}
	}

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
	return fmt.Sprintf("%s://%s", conn.transport.Scheme(), conn.c.LocalAddress())
}

func (conn *connection) RemoteAddress() string {
	return fmt.Sprintf("%s://%s", conn.transport.Scheme(), conn.c.RemoteAddress())
}

func NewPrimitiveConn(c net.Conn) PrimitiveConnection {
	return &primitiveConn{c}
}

// NewConnection allocates a new Connection using the supplied net.Conn
func NewConnection(transport Transport, pc PrimitiveConnection, opts options.Options) (Connection, error) {
	if OptionConnRawMode.Value(opts.GetOptionDefault(OptionConnRawMode, false)) {
		return newRawConnection(transport, pc, opts)
	}

	conn := &connection{
		transport: transport,
		raw:       false,
		c:         pc,
	}

	if val, ok := opts.GetOption(OptionMaxRecvMsgSize); ok {
		conn.maxrx = OptionMaxRecvMsgSize.Value(val)
	}

	return conn, nil
}
