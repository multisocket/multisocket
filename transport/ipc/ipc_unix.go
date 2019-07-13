// +build !windows,!nacl,!plan9

// Package ipc implements the IPC transport on top of UNIX domain sockets.
package ipc

import (
	"net"
	"os"
	"sync"

	"github.com/webee/multisocket/errs"
	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/transport"
)

type (
	dialer struct {
		addr *net.UnixAddr
	}

	listener struct {
		addr     *net.UnixAddr
		listener *net.UnixListener
		sync.Mutex
		closedq chan struct{}
	}
)

func (d *dialer) Dial(opts options.Options) (_ transport.Connection, err error) {
	conn, err := net.DialUnix("unix", nil, d.addr)
	if err != nil {
		return nil, err
	}
	return transport.NewConnection(Transport, conn, false)
}

func (l *listener) Listen(opts options.Options) error {
	select {
	case <-l.closedq:
		return errs.ErrClosed
	default:
	}

	// remove exists socket file
	path := l.addr.String()
	if stat, err := os.Stat(path); err == nil {
		if stat.Mode()&os.ModeSocket != 0 {
			if err := os.Remove(path); err != nil {
				return errs.ErrAddrInUse
			}
		} else {
			return errs.ErrBadAddr
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	listener, err := net.ListenUnix("unix", l.addr)
	if err != nil {
		return err
	}
	l.listener = listener
	return nil
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

	conn, err := l.listener.AcceptUnix()
	if err != nil {
		return nil, err
	}
	return transport.NewConnection(Transport, conn, true)
}

// Close implements the PipeListener Close method.
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

func (t ipcTran) NewDialer(address string) (transport.Dialer, error) {
	var (
		err  error
		addr *net.UnixAddr
	)

	if address, err = transport.StripScheme(t, address); err != nil {
		return nil, err
	}

	if addr, err = net.ResolveUnixAddr("unix", address); err != nil {
		return nil, err
	}

	d := &dialer{
		addr: addr,
	}
	return d, nil
}

// NewListener implements the Transport NewListener method.
func (t ipcTran) NewListener(address string) (transport.Listener, error) {
	var (
		err  error
		addr *net.UnixAddr
	)

	if address, err = transport.StripScheme(t, address); err != nil {
		return nil, err
	}

	if addr, err = net.ResolveUnixAddr("unix", address); err != nil {
		return nil, err
	}

	l := &listener{
		addr: addr,
	}

	return l, nil
}
