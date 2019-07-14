package tcp

import (
	"fmt"
	"net"
	"sync"

	"github.com/webee/multisocket/errs"

	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/transport"
)

type (
	tcpTran string

	dialer struct {
		addr *net.TCPAddr
	}

	listener struct {
		addr     *net.TCPAddr
		bound    net.Addr
		listener *net.TCPListener
		sync.Mutex
		closedq chan struct{}
	}
)

const (
	// Transport is a transport.Transport for TCP.
	Transport = tcpTran("tcp")
)

func init() {
	transport.RegisterTransport(Transport)
}

func configTCP(conn *net.TCPConn, opts options.Options) error {
	if val, ok := opts.GetOption(Options.NoDelay); ok {
		if err := conn.SetNoDelay(Options.NoDelay.Value(val)); err != nil {
			return err
		}
	}
	if val, ok := opts.GetOption(Options.KeepAlive); ok {
		keepAlive := Options.KeepAlive.Value(val)
		if err := conn.SetKeepAlive(keepAlive); err != nil {
			return err
		}
		if keepAlive {
			if val, ok := opts.GetOption(Options.KeepAlivePeriod); ok {
				if err := conn.SetKeepAlivePeriod(Options.KeepAlivePeriod.Value(val)); err != nil {
					return err
				}
			}
		}
	}
	if val, ok := opts.GetOption(Options.ReadBuffer); ok {
		if err := conn.SetWriteBuffer(Options.ReadBuffer.Value(val)); err != nil {
			return err
		}
	}
	if val, ok := opts.GetOption(Options.WriteBuffer); ok {
		if err := conn.SetWriteBuffer(Options.WriteBuffer.Value(val)); err != nil {
			return err
		}
	}
	return nil
}

func (d *dialer) Dial(opts options.Options) (_ transport.Connection, err error) {
	conn, err := net.DialTCP("tcp", nil, d.addr)
	if err != nil {
		return nil, err
	}
	if err = configTCP(conn, opts); err != nil {
		conn.Close()
		return nil, err
	}

	return transport.NewConnection(Transport, conn, false)
}

func (l *listener) Listen(opts options.Options) (err error) {
	select {
	case <-l.closedq:
		return errs.ErrClosed
	default:
	}

	l.listener, err = net.ListenTCP("tcp", l.addr)
	if err == nil {
		l.bound = l.listener.Addr()
	}
	return
}

func (l *listener) Accept(opts options.Options) (transport.Connection, error) {
	select {
	case <-l.closedq:
		return nil, errs.ErrClosed
	default:
	}

	if l.listener == nil {
		return nil, errs.ErrBadOperateState
	}

	conn, err := l.listener.AcceptTCP()
	if err != nil {
		return nil, err
	}
	if err = configTCP(conn, opts); err != nil {
		conn.Close()
		return nil, err
	}
	return transport.NewConnection(Transport, conn, true)
}

func (l *listener) Address() string {
	if b := l.bound; b != nil {
		return fmt.Sprintf("%s://%s", Transport.Scheme(), b.String())
	}
	return fmt.Sprintf("%s://%s", Transport.Scheme(), l.addr.String())
}

func (l *listener) Close() error {
	l.Lock()
	select {
	case <-l.closedq:
		l.Unlock()
		return errs.ErrClosed
	default:
		close(l.closedq)
	}
	l.Unlock()

	if l.listener == nil {
		return nil
	}
	return l.listener.Close()
}

func (t tcpTran) Scheme() string {
	return string(t)
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

	d := &dialer{addr: addr}

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

	l := &listener{
		addr:    addr,
		closedq: make(chan struct{}),
	}

	return l, nil
}
