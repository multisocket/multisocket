package transport

import (
	"net"

	"github.com/webee/multisocket/options"
)

// rawConnection implements the raw Connection interface on top of net.Conn.
type rawConnection struct {
	connection
	payload []byte
}

// Recv implements the TranPipe Recv method.
func (conn *rawConnection) Recv() (msg []byte, err error) {
	var (
		n int
	)

	if n, err = conn.c.Read(conn.payload); n == 0 {
		return
	}
	msg = conn.payload[:n]
	err = nil
	return
}

// Send implements the Pipe Send method.
func (conn *rawConnection) Send(msg []byte, extras ...[]byte) (err error) {
	var (
		buff = net.Buffers{}
	)

	buff = append(buff, msg)
	buff = append(buff, extras...)

	if _, err := buff.WriteTo(conn.c); err != nil {
		return err
	}

	return nil
}

// NewRawConnection allocates a new Connection using the supplied net.Conn
func NewRawConnection(transport Transport, c net.Conn, opts options.Options) (Connection, error) {
	conn := &rawConnection{
		connection: connection{
			transport: transport,
			raw:       true,
			c:         c,
		},
	}

	conn.maxrx = OptionMaxRecvMsgSize.Value(opts.GetOptionDefault(OptionMaxRecvMsgSize, uint32(4*1024)))
	conn.payload = make([]byte, conn.maxrx)

	return conn, nil
}
