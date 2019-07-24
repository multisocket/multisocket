package inproc

import (
	"fmt"
	"net"
	"sync"

	"github.com/multisocket/multisocket/options"

	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/transport"
)

type (
	listeners struct {
		sync.RWMutex
		// Who is listening, on which "address"?
		byAddr map[string]*listener
	}

	// NewPipeFunc create a pipe
	NewPipeFunc func(laddr, raddr net.Addr, opts options.Options) (net.Conn, net.Conn)
	// Tran is inproc transport
	Tran struct {
		name      string
		newPipe   NewPipeFunc
		listeners *listeners
	}

	dialer struct {
		t    *Tran
		addr string
	}

	listener struct {
		t             *Tran
		acceptedCount uint64
		addr          string
		accepts       chan chan net.Conn
		sync.Mutex
		closedq chan struct{}
	}
)

const (
	defaultAcceptQueueSize = 8
)

// NewTransport create a inproc transport
func NewTransport(name string, newPipe NewPipeFunc) *Tran {
	return &Tran{
		name:    name,
		newPipe: newPipe,
		listeners: &listeners{
			byAddr: make(map[string]*listener),
		},
	}
}

// dialer

func (d *dialer) Dial(opts options.Options) (transport.Connection, error) {
	var (
		l  *listener
		ok bool
	)

	if l, ok = d.t.getListenerByAddr(d.addr); !ok {
		return nil, transport.ErrConnRefused
	}

	ac := make(chan net.Conn)
	select {
	case <-l.closedq:
		return nil, transport.ErrConnRefused
	case l.accepts <- ac:
	}

	select {
	case <-l.closedq:
		return nil, transport.ErrConnRefused
	case dc := <-ac:
		return transport.NewConnection(d.t, dc, false)
	}
}

// listener

func (l *listener) Listen(opts options.Options) error {
	select {
	case <-l.closedq:
		return errs.ErrClosed
	default:
	}

	if ok, err := l.t.addListener(l); !ok {
		return err
	}
	l.accepts = make(chan chan net.Conn, defaultAcceptQueueSize)

	return nil
}

func (l *listener) Accept(opts options.Options) (transport.Connection, error) {
	if !l.t.isListening(l) {
		// not in listening
		return nil, transport.ErrNotListening
	}

	select {
	case <-l.closedq:
		return nil, errs.ErrClosed
	case ac := <-l.accepts:
		l.acceptedCount++
		laddr := transport.NewAddress(l.t.name, l.addr)
		raddr := transport.NewAddress(l.t.name, fmt.Sprintf("%s.dialer#%d", l.addr, l.acceptedCount))
		lc, rc := l.t.newPipe(laddr, raddr, opts)

		// notify dialer
		select {
		case <-l.closedq:
			return nil, errs.ErrClosed
		case ac <- rc:
		}

		return transport.NewConnection(l.t, lc, true)
	}
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

	l.t.removeListener(l)

	return nil
}

// inprocTran

func (t *Tran) getListenerByAddr(addr string) (l *listener, ok bool) {
	t.listeners.RLock()
	l, ok = t.listeners.byAddr[addr]
	t.listeners.RUnlock()
	return
}

func (t *Tran) isListening(l *listener) bool {
	t.listeners.RLock()
	defer t.listeners.RUnlock()

	return t.listeners.byAddr[l.addr] == l
}

func (t *Tran) addListener(l *listener) (ok bool, err error) {
	t.listeners.Lock()
	if xl, exists := t.listeners.byAddr[l.addr]; exists {
		t.listeners.Unlock()
		if xl != l {
			err = errs.ErrAddrInUse
			return
		}
		// already in listening
		return
	}

	t.listeners.byAddr[l.addr] = l
	t.listeners.Unlock()

	ok = true
	return
}

func (t *Tran) removeListener(l *listener) {
	t.listeners.Lock()
	if t.listeners.byAddr[l.addr] == l {
		delete(t.listeners.byAddr, l.addr)
	}
	t.listeners.Unlock()
}

// Scheme transport's scheme
func (t *Tran) Scheme() string {
	return t.name
}

// NewDialer create a dialer to addr
func (t *Tran) NewDialer(addr string) (transport.Dialer, error) {
	var err error
	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	d := &dialer{t: t, addr: addr}
	return d, nil
}

// NewListener create a listener on addr
func (t *Tran) NewListener(addr string) (transport.Listener, error) {
	var err error
	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	l := &listener{
		t:       t,
		addr:    addr,
		closedq: make(chan struct{}),
	}
	return l, nil
}
