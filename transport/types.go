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
		RawConn() net.Conn
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

	// Address is transport Connection's address as net.Addr
	Address struct {
		scheme string
		addr   string
	}
)

// NewAddress create an address
func NewAddress(scheme, addr string) *Address {
	return &Address{scheme, addr}
}

// Network address's network
func (a *Address) Network() string {
	return a.scheme
}

// Network address's value
func (a *Address) String() string {
	return a.addr
}
