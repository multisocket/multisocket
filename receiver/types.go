package receiver

import (
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/message"
	"github.com/webee/multisocket/options"
)

type (
	Connector = connector.Connector
	Pipe      = connector.Pipe
	PipeEvent = connector.PipeEvent

	Message = message.Message
	MsgPath = message.MsgPath

	// ReceiverAction is receiver's action
	ReceiverAction interface {
		RecvMsg() (*Message, error)
		Recv() ([]byte, error)
		// RecvContent()(*Content, error) // can free
	}

	// Receiver controls socket's recv.
	Receiver interface {
		options.Options
		AttachConnector(Connector)
		ReceiverAction
		Close()
	}
)

const (
	SendTypeToOne  = message.SendTypeToOne
	SendTypeToAll  = message.SendTypeToAll
	SendTypeToDest = message.SendTypeToDest

	PipeEventAdd    = connector.PipeEventAdd
	PipeEventRemove = connector.PipeEventRemove
)
