package multisocket

import (
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/receiver"
	"github.com/webee/multisocket/sender"
)

type (
	// Socket is a network peer
	Socket interface {
		connector.ConnectorAction
		sender.SenderAction
		receiver.ReceiverAction

		Close() error
	}

	Connector = connector.Connector
	Sender    = sender.Sender
	Receiver  = receiver.Receiver
)
