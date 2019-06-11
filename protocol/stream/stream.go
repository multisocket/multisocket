package stream

import (
	"io"

	"github.com/webee/multisocket"
)

type (
	// Stream is a stream
	Stream interface {
		multisocket.ConnectorAction
		Close() error

		// actions
		SendStreamTo(dest multisocket.MsgPath, content io.Reader) error // for reply send
		SendStream(content io.Reader) error                             // for initiative send

		RecvStream() (src multisocket.MsgPath, content io.ReadCloser, err error)
	}
)
