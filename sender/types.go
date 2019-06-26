package sender

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

	// SenderAction is sender's action
	SenderAction interface {
		Send(content []byte) error                 // for initiative send one
		SendTo(dest MsgPath, content []byte) error // for reply send
		SendAll(content []byte) error              // for initiative send all
		SendMsg(msg *Message) error                // for forward message
	}

	// Sender controls socket's send.
	Sender interface {
		options.Options
		AttachConnector(Connector)
		SenderAction
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
