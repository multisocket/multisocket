package sender

import (
	"github.com/multisocket/multisocket/connector"
	"github.com/multisocket/multisocket/message"
	"github.com/multisocket/multisocket/options"
)

type (
	// Action is sender's action
	Action interface {
		Send(content []byte) error                         // for initiative send one
		SendTo(dest message.MsgPath, content []byte) error // for reply send
		SendAll(content []byte) error                      // for initiative send all
		SendMsg(msg *message.Message) error                // for forward message
	}

	// Sender controls socket's send.
	Sender interface {
		options.Options
		AttachConnector(connector.Connector)
		Action
		Close()
	}
)
