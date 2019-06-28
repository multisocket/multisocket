package receiver

import (
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/message"
	"github.com/webee/multisocket/options"
)

type (
	// Action is receiver's action
	Action interface {
		RecvMsg() (*message.Message, error)
		Recv() ([]byte, error)
		// RecvContent()(*Content, error) // can free
	}

	// Receiver controls socket's recv.
	Receiver interface {
		options.Options
		AttachConnector(connector.Connector)
		Action
		Close()
	}
)
