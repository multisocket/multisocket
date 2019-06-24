package transport

import (
	"github.com/webee/multisocket/options"
)

type (
	// Connection is connection between peers.
	Connection interface {
		Transport() Transport
		IsRaw() bool

		Send(msg []byte, extras ...[]byte) error
		Recv() ([]byte, error)

		Close() error

		LocalAddress() string
		RemoteAddress() string
	}

	// Dialer is dialer
	Dialer interface {
		Dial(opts options.Options) (Connection, error)
	}

	// Listener is listener
	Listener interface {
		Listen(opts options.Options) error
		Accept(opts options.Options) (Connection, error)
		Close() error
	}

	// Transport is transport
	Transport interface {
		Scheme() string
		NewDialer(address string) (Dialer, error)
		NewListener(address string) (Listener, error)
	}
)
