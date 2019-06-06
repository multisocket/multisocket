package connector

import (
	"reflect"
	"sync"

	"github.com/webee/multisocket/options"

	log "github.com/sirupsen/logrus"

	"github.com/webee/multisocket"
	"github.com/webee/multisocket/socket"
	"github.com/webee/multisocket/transport"
)

type (
	connector struct {
		limit int

		sync.Mutex
		dialers        []*dialer
		listeners      []*listener
		pipes          map[*pipe]struct{}
		pipeEventHooks map[uintptr]socket.PipeEventHook
		closed         bool
	}
)

const (
	// defaultMaxRxSize is the default maximum Rx size
	defaultMaxRxSize = 1024 * 1024
)

// New create a any Connector
func New() socket.Connector {
	return NewWithLimit(-1)
}

// NewWithLimit create a Connector with limited connections.
func NewWithLimit(n int) socket.Connector {
	c := &connector{
		limit:          n,
		dialers:        make([]*dialer, 0),
		listeners:      make([]*listener, 0),
		pipes:          make(map[*pipe]struct{}),
		pipeEventHooks: make(map[uintptr]socket.PipeEventHook),
	}

	return c
}

func (c *connector) addPipe(p *pipe) {
	log.WithField("domain", "connector").
		WithFields(log.Fields{"id": p.ID(), "localAddress": p.LocalAddress(), "remoteAddress": p.RemoteAddress()}).
		Debug("add pipe")

	c.Lock()
	defer c.Unlock()

	if c.limit == -1 || c.limit > len(c.pipes) {
		c.pipes[p] = struct{}{}
		for _, hook := range c.pipeEventHooks {
			go hook(socket.PipeEventAdd, p)
		}
	} else {
		go p.Close()
	}

	// check exceed limit
	if c.limit != -1 && c.limit <= len(c.pipes) {
		// stop connect
		for _, l := range c.listeners {
			l.stop()
		}

		for _, d := range c.dialers {
			d.stop()
		}
	}
}

func (c *connector) remPipe(p *pipe) {
	log.WithField("domain", "connector").
		WithFields(log.Fields{"id": p.ID()}).
		Debug("remove pipe")

	c.Lock()
	delete(c.pipes, p)
	c.Unlock()

	// If the pipe was from a dialer, inform it so that it can redial.
	if d := p.d; d != nil {
		go d.pipeClosed()
	}

	c.Lock()
	defer c.Unlock()
	if c.limit != -1 && c.limit > len(c.pipes) {
		// below limit

		// start connect
		for _, l := range c.listeners {
			l.start()
		}

		for _, d := range c.dialers {
			d.start()
		}
	}
}

func (c *connector) Dial(addr string) error {
	return c.DialOptions(addr, options.NewOptions())
}

func (c *connector) DialOptions(addr string, opts options.Options) error {
	d, err := c.NewDialer(addr, opts)
	if err != nil {
		return err
	}
	return d.Dial()
}

func (c *connector) NewDialer(addr string, opts options.Options) (d socket.Dialer, err error) {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		err = multisocket.ErrClosed
		return
	}

	var (
		t  transport.Transport
		td transport.Dialer
	)

	if t = transport.GetTransportFromAddr(addr); t == nil {
		err = multisocket.ErrBadTran
		return
	}

	if td, err = t.NewDialer(addr); err != nil {
		return
	}
	td.SetOption(transport.OptionMaxRecvSize, defaultMaxRxSize)

	xd := newDialer(c, td)
	if c.limit != -1 && c.limit <= len(c.pipes) {
		// exceed limit
		xd.stop()
	}
	d = xd
	for _, ov := range opts.OptionValues() {
		if err = d.SetOption(ov.Option, ov.Value); err != nil {
			return
		}
	}

	c.dialers = append(c.dialers, xd)
	return d, nil
}

func (c *connector) Listen(addr string) error {
	return c.ListenOptions(addr, options.NewOptions())
}

func (c *connector) ListenOptions(addr string, opts options.Options) error {
	l, err := c.NewListener(addr, opts)
	if err != nil {
		return err
	}
	return l.Listen()
}

func (c *connector) NewListener(addr string, opts options.Options) (l socket.Listener, err error) {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		err = multisocket.ErrClosed
		return
	}

	var (
		t  transport.Transport
		tl transport.Listener
	)

	if t = transport.GetTransportFromAddr(addr); t == nil {
		err = multisocket.ErrBadTran
		return
	}

	if tl, err = t.NewListener(addr); err != nil {
		return
	}
	tl.SetOption(transport.OptionMaxRecvSize, defaultMaxRxSize)

	for _, ov := range opts.OptionValues() {
		if err = tl.SetOption(ov.Option, ov.Value); err != nil {
			tl.Close()
			return
		}
	}

	xl := newListener(c, tl)
	if c.limit != -1 && c.limit <= len(c.pipes) {
		// exceed limit
		xl.stop()
	}
	l = xl

	c.listeners = append(c.listeners, xl)

	return
}

func (c *connector) Close() {
	c.Lock()
	if c.closed {
		c.Unlock()
		return
	}
	listeners := c.listeners
	dialers := c.dialers
	pipes := c.pipes

	c.listeners = nil
	c.dialers = nil
	c.pipes = nil
	c.Unlock()

	for _, l := range listeners {
		l.Close()
	}
	for _, d := range dialers {
		d.Close()
	}

	for p := range pipes {
		p.Close()
	}
}

func (c *connector) RegisterPipeEventHook(hook socket.PipeEventHook) {
	c.Lock()
	c.pipeEventHooks[reflect.ValueOf(hook).Pointer()] = hook
	c.Unlock()
}

func (c *connector) UnregisterPipeEventHook(hook socket.PipeEventHook) {
	c.Lock()
	delete(c.pipeEventHooks, reflect.ValueOf(hook).Pointer())
	c.Unlock()
}
