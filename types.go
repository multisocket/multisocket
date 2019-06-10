package multisocket

import (
	"github.com/webee/multisocket/options"
)

type (
	// Socket is a network peer
	Socket interface {
		Connector() Connector
		Sender() Sender
		Receiver() Receiver

		ConnectorAction
		SenderAction
		ReceiverAction

		Close() error
	}

	// Dialer is for connecting a listening socket.
	Dialer interface {
		options.Options

		Dial() error
		Close() error
	}

	// Listener is for listening and accepting connections.
	Listener interface {
		options.Options

		Listen() error
		Close() error
	}
)

type (
	// Pipe is a connection between two peers.
	Pipe interface {
		ID() uint32
		LocalAddress() string
		RemoteAddress() string

		Send(msgs ...[]byte) error
		Recv() ([]byte, error)

		Close()
	}
)

type (
	// PipeEvent is pipe event
	PipeEvent int

	// PipeEventHandler can handle pipe event
	PipeEventHandler interface {
		HandlePipeEvent(PipeEvent, Pipe)
	}
)

// pipe events
const (
	PipeEventAdd PipeEvent = iota
	PipeEventRemove
)

type (
	// ConnectorAction is connector's action
	ConnectorAction interface {
		Dial(addr string) error
		DialOptions(addr string, opts options.Options) error
		NewDialer(addr string, opts options.Options) (Dialer, error)

		Listen(addr string) error
		ListenOptions(addr string, opts options.Options) error
		NewListener(addr string, opts options.Options) (Listener, error)
	}

	// Connector controls socket's connections
	Connector interface {
		options.Options
		ConnectorAction
		Close()
		RegisterPipeEventHandler(PipeEventHandler)
		UnregisterPipeEventHandler(PipeEventHandler)
	}

	// SenderAction is sender's action
	SenderAction interface {
		SendTo(dest MsgPath, content []byte) error // for reply send
		Send(content []byte) error                 // for initiative send one
		SendAll(content []byte) error              // for initiative send all
		SendMsg(msg *Message) error
	}

	// Sender controls socket's send.
	Sender interface {
		options.Options
		AttachConnector(Connector)
		SenderAction
		Close()
	}

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
