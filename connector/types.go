package connector

import (
	"time"

	"github.com/webee/multisocket/options"
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

	// ConnectorAction is connector's action
	ConnectorAction interface {
		SetNegotiator(Negotiator)

		Dial(addr string) error
		DialOptions(addr string, ovs options.OptionValues) error
		NewDialer(addr string, ovs options.OptionValues) (Dialer, error)
		StopDial(addr string)

		Listen(addr string) error
		ListenOptions(addr string, ovs options.OptionValues) error
		NewListener(addr string, ovs options.OptionValues) (Listener, error)
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
)
