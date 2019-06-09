package transport

import (
	"github.com/webee/multisocket/options"
)

type (
	// Connection is connection between peers.
	Connection interface {
		Transport() Transport

		Send(...[]byte) error
		Recv() ([]byte, error)

		Close() error

		LocalAddress() string
		RemoteAddress() string
	}

	// Dialer is dialer
	Dialer interface {
		options.Options

		Dial() (Connection, error)
	}

	// Listener is listener
	Listener interface {
		options.Options

		Listen() error
		Accept() (Connection, error)
		Close() error
	}

	// Transport is transport
	Transport interface {
		Scheme() string
		NewDialer(address string) (Dialer, error)
		NewListener(address string) (Listener, error)
	}
)
