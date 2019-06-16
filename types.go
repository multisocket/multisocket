package multisocket

import (
	"time"

	"github.com/webee/multisocket/options"
)

type (
	// Socket is a network peer
	Socket interface {
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

		IsRaw() bool

		Send(msg []byte, extras ...[]byte) error
		SendTimeout(deadline time.Duration, msg []byte, extras ...[]byte) error
		Recv() ([]byte, error)
		RecvTimeout(deadline time.Duration) ([]byte, error)

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
	// Negotiator is use for handshaking when adding pipe
	Negotiator interface {
		Negotiate(pipe Pipe) error
	}

	// ConnectorAction is connector's action
	ConnectorAction interface {
		SetNegotiator(Negotiator)

		Dial(addr string) error
		DialOptions(addr string, opts options.Options) error
		NewDialer(addr string, opts options.Options) (Dialer, error)
		StopDial(addr string)

		Listen(addr string) error
		ListenOptions(addr string, opts options.Options) error
		NewListener(addr string, opts options.Options) (Listener, error)
		StopListen(addr string)

		GetPipe(id uint32) Pipe
		ClosePipe(id uint32)
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
		SendTo(dest MsgPath, content []byte, extras ...[]byte) error // for reply send
		Send(content []byte, extras ...[]byte) error                 // for initiative send one
		SendAll(content []byte, extras ...[]byte) error              // for initiative send all
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
