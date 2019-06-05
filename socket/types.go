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

		Send([]byte) (PipeInfo, error)
		SendTo(PipeInfo, []byte) error
		Recv() (PipeInfo, []byte, error)
		RecvFrom(PipeInfo) ([]byte, error)

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
	// PipeInfo is pipe info
	PipeInfo interface {
		ID() uint32
		LocalAddress() string
		RemoteAddress() string
	}

	// Pipe is pipe
	Pipe interface {
		PipeInfo
		Send([]byte) error
		Recv() ([]byte, error)
		Close() error
	}
)

type (
	// Connector controls socket's connections
	Connector interface {
		Dial(addr string) error
		DialOptions(addr string, opts multisocket.Options) error
		NewDialer(addr string, opts multisocket.Options) (Dialer, error)

		Listen(addr string) error
		ListenOptions(addr string, opts multisocket.Options) error
		NewListener(addr string, opts multisocket.Options) (Listener, error)

		Close() error

		AddPipeChannel(chan<- Pipe)
		RemovePipeChannel(chan<- Pipe)
	}

	// Sender controls socket's send.
	Sender interface {
		AttachConnector(Connector)

		Send([]byte) (PipeInfo, error)
		SendTo(PipeInfo, []byte) error

		Close()
	}

	// Receiver controls socket's recv.
	Receiver interface {
		AttachConnector(Connector)

		Recv() (PipeInfo, []byte, error)
		RecvFrom(PipeInfo) ([]byte, error)

		Close()
	}
)
