package receiver

import (
	"github.com/multisocket/multisocket/connector"
	"github.com/multisocket/multisocket/message"
	"github.com/multisocket/multisocket/options"
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
