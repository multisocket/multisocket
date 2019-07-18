package connector

import (
	"github.com/multisocket/multisocket/message"
	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/transport"
)

type (
	// Pipe is a connection between two peers.
	Pipe interface {
		options.Options

		ID() uint32
		IsRaw() bool

		transport.Connection
		SendMsg(msg *message.Message) (err error)
		RecvMsg() (msg *message.Message, err error)
	}
)

type (
	// PipeEvent is pipe event
	PipeEvent int

	// PipeEventHandlerFunc can handle pipe event
	PipeEventHandlerFunc func(PipeEvent, Pipe)
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
		TransportDialer() transport.Dialer
	}

	// Listener is for listening and accepting connections.
	Listener interface {
		options.Options

		Listen() error
		Close() error
		TransportListener() transport.Listener
	}

	// CoreAction is connector's core action
	CoreAction interface {
		Dial(addr string) error
		DialOptions(addr string, ovs options.OptionValues) error
		NewDialer(addr string, ovs options.OptionValues) (Dialer, error)
		// StopDial stop dial to address, but keep connected pipes.
		StopDial(addr string)

		Listen(addr string) error
		ListenOptions(addr string, ovs options.OptionValues) error
		NewListener(addr string, ovs options.OptionValues) (Listener, error)
		// StopDial stop listen on address, but keep accepted pipes.
		StopListen(addr string)
	}

	// Action is connector's action
	Action interface {
		SetNegotiator(Negotiator)

		CoreAction

		GetPipe(id uint32) Pipe
		ClosePipe(id uint32)
	}

	// Connector controls socket's connections
	Connector interface {
		options.Options
		Action
		Close()
		SetPipeEventHandler(PipeEventHandlerFunc)
		ClearPipeEventHandler(PipeEventHandlerFunc)
	}
)
