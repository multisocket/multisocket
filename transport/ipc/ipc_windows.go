// +build windows

// Package ipc implements the IPC transport on top of Windows Named Pipes.
package ipc

import (
	"net"
	"os"
	"sync"

	"github.com/Microsoft/go-winio"
	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/transport"
)

type (
	dialer struct {
		path string
	}

	listener struct {
		path     string
		listener net.Listener
		sync.Mutex
		closedq chan struct{}
	}
)

func (d *dialer) Dial(opts options.Options) (transport.Connection, error) {
	conn, err := winio.DialPipe("\\\\.\\pipe\\"+d.path, nil)
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

	// remove exists named pipe file
	path := l.addr.String()
	if stat, err := os.Stat(path); err == nil {
		if stat.Mode()&os.ModeNamedPipe != 0 {
			if err := os.Remove(path); err != nil {
				return errs.ErrAddrInUse
			}
		} else {
			return errs.ErrBadAddr
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	config := &winio.PipeConfig{
		InputBufferSize:    Options.Listener.InputBufferSize.ValueFrom(opts),
		OutputBufferSize:   Options.Listener.OutputBufferSize.ValueFrom(opts),
		SecurityDescriptor: Options.Listener.SecurityDescriptor.ValueFrom(opts),
		MessageMode:        false,
	}

	listener, err := winio.ListenPipe("\\\\.\\pipe\\"+l.path, config)
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

	conn, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}
	return transport.NewConnection(Transport, conn, true)
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

func (t ipcTran) NewDialer(address string) (transport.Dialer, error) {
	var err error
	if address, err = transport.StripScheme(t, address); err != nil {
		return nil, err
	}

	d := &dialer{path: address}

	return d, nil
}

// NewListener implements the Transport NewListener method.
func (t ipcTran) NewListener(address string) (transport.Listener, error) {
	var err error
	if address, err = transport.StripScheme(t, address); err != nil {
		return nil, err
	}

	l := &listener{
		path:    address,
		closedq: make(chan struct{}),
	}

	return l, nil
}
