package multisocket

import (
	"github.com/multisocket/multisocket/connector"
	"github.com/multisocket/multisocket/message"
	"github.com/multisocket/multisocket/options"
)

type (
	// ConnectorAction is connector's actions
	ConnectorAction = connector.Action

	// Socket is a network peer
	Socket interface {
		options.Options

		ConnectorAction

		RecvMsg() (*message.Message, error)
		SendMsg(msg *message.Message) error             // for forward message
		Send(body []byte) error                         // for initiative send one
		SendAll(body []byte) error                      // for initiative send all
		SendTo(dest message.MsgPath, body []byte) error // for reply send

		Close() error
	}
)
