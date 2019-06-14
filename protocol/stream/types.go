package stream

import (
	"io"
	"time"

	"github.com/webee/multisocket"
	"github.com/webee/multisocket/options"
)

type (
	// Stream is the Stream protocol
	Stream interface {
		options.Options
		multisocket.ConnectorAction
		Close() error

		Connect(timeout time.Duration) (conn Connection, err error)
		Accept() (conn Connection, err error)
	}

	// Connection is one stream connection between two peer
	Connection interface {
		io.ReadWriteCloser
	}
)

// control messages
const (
	ControlMsgKeepAlive    string = ">"
	ControlMsgKeepAliveAck string = "<"
)
