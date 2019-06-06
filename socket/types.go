package socket

import (
	"github.com/webee/multisocket"
)

type (
	// Socket is a network peer
	Socket interface {
		Dial(addr string) error
		DialOptions(addr string, opts multisocket.Options) error
		NewDialer(addr string, opts multisocket.Options) (Dialer, error)

		Listen(addr string) error
		ListenOptions(addr string, opts multisocket.Options) error
		NewListener(addr string, opts multisocket.Options) (Listener, error)

		SendTo(src MsgSource, content []byte) error
		SendMsg(msg *Message) error
		Send([]byte) error
		RecvMsg() (*Message, error)
		Recv() ([]byte, error)

		Close() error
	}

	// Dialer is dialer
	Dialer interface {
		multisocket.Options

		Dial() error
		Close() error
	}

	// Listener is listener
	Listener interface {
		multisocket.Options

		Listen() error
		Close() error
	}
)

type (
	// Pipe is pipe
	Pipe interface {
		ID() uint32
		LocalAddress() string
		RemoteAddress() string

		Send(msgs ...[]byte) error
		Recv() ([]byte, error)

		Close() error
	}
)

// PipeEvent is pipe event
type PipeEvent int

// pipe events
const (
	PipeEventAdd PipeEvent = iota
	PipeEventRemove
)

// PipeEventHook is pipe event hook
type PipeEventHook func(PipeEvent, Pipe)

type (
	// Connector controls socket's connections
	Connector interface {
		Dial(addr string) error
		DialOptions(addr string, opts multisocket.Options) error
		NewDialer(addr string, opts multisocket.Options) (Dialer, error)

		Listen(addr string) error
		ListenOptions(addr string, opts multisocket.Options) error
		NewListener(addr string, opts multisocket.Options) (Listener, error)

		Close()

		RegisterPipeEventHook(PipeEventHook)
		UnregisterPipeEventHook(PipeEventHook)
	}

	// Sender controls socket's send.
	Sender interface {
		AttachConnector(Connector)

		SendTo(src MsgSource, content []byte) error
		SendMsg(msg *Message) error
		Send(content []byte) error

		Close()
	}

	// Receiver controls socket's recv.
	Receiver interface {
		AttachConnector(Connector)

		RecvMsg() (*Message, error)
		Recv() ([]byte, error)

		Close()
	}
)
