package transport

import (
	"net"
	"strings"
	"sync"

	"github.com/webee/multisocket/options"
)

// Connection is connection between peers.
type Connection interface {
	Send(...[]byte) error
	Recv() ([]byte, error)
	Close() error

	LocalAddress() string
	RemoteAddress() string
}

// Dialer is dialer
type Dialer interface {
	options.Options

	Dial() (Connection, error)
}

// Listener is listener
type Listener interface {
	options.Options

	Listen() error
	Accept() (Connection, error)
	Close() error
}

// Transport is transport
type Transport interface {
	Scheme() string
	NewDialer(address string) (Dialer, error)
	NewListener(address string) (Listener, error)
}

// StripScheme removes the leading scheme (such as "http://") from an address
// string.  This is mostly a utility for benefit of transport providers.
func StripScheme(t Transport, addr string) (string, error) {
	if !strings.HasPrefix(addr, t.Scheme()+"://") {
		return addr, ErrBadTran
	}
	return addr[len(t.Scheme()+"://"):], nil
}

// ParseScheme parse scheme from address
func ParseScheme(addr string) (scheme string) {
	var i int

	if i = strings.Index(addr, "://"); i < 0 {
		return
	}

	scheme = addr[:i]
	return
}

// ResolveTCPAddr is like net.ResolveTCPAddr, but it handles the
// wildcard used in nanomsg URLs, replacing it with an empty
// string to indicate that all local interfaces be used.
func ResolveTCPAddr(addr string) (*net.TCPAddr, error) {
	if strings.HasPrefix(addr, "*") {
		addr = addr[1:]
	}
	return net.ResolveTCPAddr("tcp", addr)
}

var (
	lock       sync.RWMutex
	transports = map[string]Transport{}
)

// GetTransportFromAddr get transport for the address scheme
func GetTransportFromAddr(addr string) Transport {
	return GetTransport(ParseScheme(addr))
}

// RegisterTransport is used to register the transport globally,
// after which it will be available for all sockets.  The
// transport will override any others registered for the same
// scheme.
func RegisterTransport(t Transport) {
	lock.Lock()
	defer lock.Unlock()
	transports[t.Scheme()] = t
}

// GetTransport is used by a socket to lookup the transport
// for a given scheme.
func GetTransport(scheme string) Transport {
	lock.RLock()
	defer lock.RUnlock()
	if t, ok := transports[scheme]; ok {
		return t
	}
	return nil
}
