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

	Message   = message.Message
	MsgHeader = message.MsgHeader
	MsgPath   = message.MsgPath

	// ReceiverAction is receiver's action
	ReceiverAction interface {
		RecvMsg() (*Message, error)
		Recv() ([]byte, error)
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
