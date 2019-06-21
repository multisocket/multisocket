package stream

import (
	"io"
	"time"

	"github.com/webee/multisocket/connector"

	"github.com/webee/multisocket/options"
)

type (
	// Stream is the Stream protocol
	Stream interface {
		options.Options
		connector.ConnectorCoreAction
		Close() error

		Connect(timeout time.Duration) (conn Connection, err error)
		Accept() (conn Connection, err error)
	}

	// Connection is one stream connection between two peer
	Connection interface {
		io.ReadWriteCloser
		Closed() bool
	}
)

// control messages
const (
	ControlMsgKeepAlive    string = ">"
	ControlMsgKeepAliveAck string = "<"
)
