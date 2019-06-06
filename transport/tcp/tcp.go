// Package tcp implements the TCP transport . To enable it simply import it.
package tcp

import (
	"net"

	"github.com/webee/multisocket"
	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/transport"
)

const (
	// Transport is a transport.Transport for TCP.
	Transport = tcpTran(0)
)

func init() {
	transport.RegisterTransport(Transport)
}

func configTCP(conn *net.TCPConn, opts options.Options) error {
	if val, ok := opts.GetOption(OptionNoDelay); ok {
		if err := conn.SetNoDelay(OptionNoDelay.Value(val)); err != nil {
			return err
		}
	}
	if val, ok := opts.GetOption(OptionKeepAlive); ok {
		if err := conn.SetKeepAlive(OptionKeepAlive.Value(val)); err != nil {
			return err
		}
	}
	if val, ok := opts.GetOption(OptionKeepAliveTime); ok {
		if err := conn.SetKeepAlivePeriod(OptionKeepAliveTime.Value(val)); err != nil {
			return err
		}
	}
	return nil
}

type dialer struct {
	options.Options

	addr string
}

func (d *dialer) Dial() (_ transport.Connection, err error) {
	var (
		addr *net.TCPAddr
	)

	if addr, err = transport.ResolveTCPAddr(d.addr); err != nil {
		return nil, err
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return nil, err
	}
	if err = configTCP(conn, d.Options); err != nil {
		conn.Close()
		return nil, err
	}

	return transport.NewConnection(Transport.Scheme(), conn, d.Options)
}

type listener struct {
	options.Options

	addr     *net.TCPAddr
	bound    net.Addr
	listener *net.TCPListener
}

func (l *listener) Accept() (transport.Connection, error) {

	if l.listener == nil {
		return nil, multisocket.ErrClosed
	}
	conn, err := l.listener.AcceptTCP()
	if err != nil {
		return nil, err
	}
	if err = configTCP(conn, l.Options); err != nil {
		conn.Close()
		return nil, err
	}
	return transport.NewConnection(Transport.Scheme(), conn, l.Options)
}

func (l *listener) Listen() (err error) {
	l.listener, err = net.ListenTCP("tcp", l.addr)
	if err == nil {
		l.bound = l.listener.Addr()
	}
	return
}

func (l *listener) Address() string {
	if b := l.bound; b != nil {
		return "tcp://" + b.String()
	}
	return "tcp://" + l.addr.String()
}

func (l *listener) Close() error {
	l.listener.Close()
	return nil
}

type tcpTran int

func (t tcpTran) Scheme() string {
	return "tcp"
}

func newDefaultOptions() options.Options {
	// default options
	return options.NewOptions().
		WithOption(OptionNoDelay, true).
		WithOption(OptionKeepAlive, true).
		WithOption(transport.OptionMaxRecvSize, 0)
}

func (t tcpTran) NewDialer(addr string) (transport.Dialer, error) {
	var err error
	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	// check to ensure the provided addr resolves correctly.
	if _, err = transport.ResolveTCPAddr(addr); err != nil {
		return nil, err
	}

	d := &dialer{Options: newDefaultOptions(), addr: addr}

	return d, nil
}

func (t tcpTran) NewListener(addr string) (transport.Listener, error) {
	var err error
	l := &listener{Options: newDefaultOptions()}

	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	if l.addr, err = transport.ResolveTCPAddr(addr); err != nil {
		return nil, err
	}

	return l, nil
}
