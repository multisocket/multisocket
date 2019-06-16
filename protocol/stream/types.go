package stream

import (
	"io"
	"time"

	"github.com/webee/multisocket/options"
	. "github.com/webee/multisocket/types"
)

type (
	// Stream is the Stream protocol
	Stream interface {
		options.Options
		ConnectorAction
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
