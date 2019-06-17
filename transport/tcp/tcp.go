package tcp

import (
	"fmt"
	"net"

	"github.com/webee/multisocket/errs"

	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/transport"
)

type (
	tcpTran int

	dialer struct {
		options.Options

		addr *net.TCPAddr
	}

	listener struct {
		options.Options

		addr     *net.TCPAddr
		bound    net.Addr
		listener *net.TCPListener
	}
)

const (
	// Transport is a transport.Transport for TCP.
	Transport = tcpTran(0)
	scheme    = "tcp"
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

func (d *dialer) Dial() (_ transport.Connection, err error) {
	conn, err := net.DialTCP(scheme, nil, d.addr)
	if err != nil {
		return nil, err
	}
	if err = configTCP(conn, d.Options); err != nil {
		conn.Close()
		return nil, err
	}

	return transport.NewConnection(Transport, transport.NewPrimitiveConn(conn), d.Options)
}

func (l *listener) Listen() (err error) {
	l.listener, err = net.ListenTCP(scheme, l.addr)
	if err == nil {
		l.bound = l.listener.Addr()
	}
	return
}

func (l *listener) Accept() (transport.Connection, error) {
	if l.listener == nil {
		return nil, errs.ErrBadOperateState
	}

	conn, err := l.listener.AcceptTCP()
	if err != nil {
		return nil, err
	}
	if err = configTCP(conn, l.Options); err != nil {
		conn.Close()
		return nil, err
	}
	return transport.NewConnection(Transport, transport.NewPrimitiveConn(conn), l.Options)
}

func (l *listener) Address() string {
	if b := l.bound; b != nil {
		return fmt.Sprintf("%s://%s", scheme, b.String())
	}
	return fmt.Sprintf("%s://%s", scheme, l.addr.String())
}

func (l *listener) Close() error {
	if l.listener == nil {
		return nil
	}
	return l.listener.Close()
}

func (t tcpTran) Scheme() string {
	return scheme
}

func newDefaultOptions() options.Options {
	// default options
	return options.NewOptions().
		WithOption(OptionNoDelay, true).
		WithOption(OptionKeepAlive, true).
		WithOption(transport.OptionMaxRecvMsgSize, 1024*1024)
}

func (t tcpTran) NewDialer(address string) (transport.Dialer, error) {
	var (
		err  error
		addr *net.TCPAddr
	)
	if address, err = transport.StripScheme(t, address); err != nil {
		return nil, err
	}

	if addr, err = transport.ResolveTCPAddr(address); err != nil {
		return nil, err
	}

	d := &dialer{Options: newDefaultOptions(), addr: addr}

	return d, nil
}

func (t tcpTran) NewListener(address string) (transport.Listener, error) {
	var (
		err  error
		addr *net.TCPAddr
	)

	if address, err = transport.StripScheme(t, address); err != nil {
		return nil, err
	}

	if addr, err = transport.ResolveTCPAddr(address); err != nil {
		return nil, err
	}

	l := &listener{Options: newDefaultOptions(), addr: addr}

	return l, nil
}
