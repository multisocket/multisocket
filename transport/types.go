package transport

import (
	"net"

	"github.com/multisocket/multisocket/options"
)

type (
	// Connection is connection between peers.
	Connection interface {
		Transport() Transport
		net.Conn
		Writev(v ...[]byte) (int64, error)
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
