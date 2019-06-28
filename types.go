package multisocket

import (
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/receiver"
	"github.com/webee/multisocket/sender"
)

type (
	// Socket is a network peer
	Socket interface {
		connector.Action
		sender.Action
		receiver.Action

		Close() error

		GetConnector() connector.Connector
		GetSender() sender.Sender
		GetReceiver() receiver.Receiver
	}
)
