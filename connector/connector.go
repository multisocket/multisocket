package connector

import (
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/transport"
)

type (
	connector struct {
		options.Options

		sync.Mutex
		negotiator       Negotiator
		limit            int
		dialers          map[*dialer]struct{} // can dial to any address any times
		listeners        map[*listener]struct{}
		pipes            map[uint32]*pipe
		pipeEventHandler PipeEventHandlerFunc
		closed           bool
	}
)

// NewWithOptionValues create a Connector with option values
func NewWithOptionValues(ovs options.OptionValues) Connector {
	return NewWithLimitAndOptionValues(-1, ovs)
}

// NewWithLimitAndOptionValues create a Connector with limit and option values
func NewWithLimitAndOptionValues(limit int, ovs options.OptionValues) Connector {
	return NewWithLimitAndOptions(limit, options.NewOptionsWithValues(ovs))
}

// NewWithOptions create a Connector with options
func NewWithOptions(opts options.Options) Connector {
	return NewWithLimitAndOptions(-1, opts)
}

// NewWithLimitAndOptions create a Connector with limit and options
func NewWithLimitAndOptions(limit int, opts options.Options) Connector {
	c := &connector{
		Options:   opts,
		limit:     limit,
		dialers:   make(map[*dialer]struct{}),
		listeners: make(map[*listener]struct{}),
		pipes:     make(map[uint32]*pipe),
	}
	c.Options.AddOptionChangeHook(c.onOptionChange)
	for o, v := range c.Options.OptionValues() {
		c.onOptionChange(o, nil, v)
	}

	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "connector").
			WithField("limit", c.limit).
			Debug("create")
	}
	return c
}

func (c *connector) onOptionChange(opt options.Option, oldVal, newVal interface{}) error {
	switch opt {
	case Options.PipeLimit:
		c.Lock()
		oldLimit := c.limit
		c.limit = Options.PipeLimit.Value(newVal)
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithField("domain", "connector").
				WithField("oldLimit", oldLimit).
				WithField("newLimit", c.limit).
				Debug("change limit")
		}
		c.checkLimit(true)
		c.Unlock()
	}
	return nil
}

// used by other functions, must get lock first
func (c *connector) checkLimit(checkNoLimit bool) {
	if c.closed {
		return
	}

	if checkNoLimit && c.limit == -1 {
		// start connecting
		for l := range c.listeners {
			l.start()
		}

		for d := range c.dialers {
			d.start()
		}
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithField("domain", "connector").
				WithFields(log.Fields{"limit": c.limit, "pipes": len(c.pipes)}).
				WithField("action", "start").
				Debug("check limit")
		}
	} else if c.limit != -1 && c.limit > len(c.pipes) {
		// below limit
		// start connecting
		for l := range c.listeners {
			l.start()
		}

		for d := range c.dialers {
			d.start()
		}
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithField("domain", "connector").
				WithFields(log.Fields{"limit": c.limit, "pipes": len(c.pipes)}).
				WithField("action", "start").
				Debug("check limit")
		}
	} else if c.limit != -1 && c.limit <= len(c.pipes) {
		// check exceed limit
		// stop connecting
		for l := range c.listeners {
			l.stop()
		}

		for d := range c.dialers {
			d.stop()
		}
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithField("domain", "connector").
				WithFields(log.Fields{"limit": c.limit, "pipes": len(c.pipes)}).
				WithField("action", "stop").
				Debug("check limit")
		}
	}
}

func (c *connector) addPipe(p *pipe) {
	c.Lock()
	defer c.Unlock()

	if c.negotiator != nil {
		// negotiating
		if err := c.negotiator.Negotiate(p); err != nil {
			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithField("domain", "connector").
					WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw(), "localAddress": p.LocalAddress(), "remoteAddress": p.RemoteAddress()}).
					WithFields(log.Fields{"limit": c.limit, "pipes": len(c.pipes)}).
					WithField("action", "netotiating").
					WithError(err).
					Error("add pipe")
			}
			return
		}
	}

	if c.limit == -1 || c.limit > len(c.pipes) {
		c.pipes[p.ID()] = p
		if c.pipeEventHandler != nil {
			c.pipeEventHandler(PipeEventAdd, p)
		}

		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithField("domain", "connector").
				WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw(), "localAddress": p.LocalAddress(), "remoteAddress": p.RemoteAddress()}).
				WithFields(log.Fields{"limit": c.limit, "pipes": len(c.pipes)}).
				Debug("add pipe")
		}

		c.checkLimit(false)
	} else {
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithField("domain", "connector").
				WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw(), "localAddress": p.LocalAddress(), "remoteAddress": p.RemoteAddress()}).
				WithFields(log.Fields{"limit": c.limit, "pipes": len(c.pipes)}).
				Debug("drop pipe")
		}

		go p.Close()
	}
}

func (c *connector) remPipe(p *pipe) {
	c.Lock()
	delete(c.pipes, p.ID())
	if c.pipeEventHandler != nil {
		c.pipeEventHandler(PipeEventRemove, p)
	}
	c.Unlock()

	if log.IsLevelEnabled(log.DebugLevel) {
		isRaw := p.GetOptionDefault(Options.Pipe.Raw)
		log.WithField("domain", "connector").
			WithFields(log.Fields{"id": p.ID(), "raw": isRaw, "localAddress": p.LocalAddress(), "remoteAddress": p.RemoteAddress()}).
			WithFields(log.Fields{"limit": c.limit, "pipes": len(c.pipes)}).
			Debug("remove pipe")
	}

	// If the pipe was from a dialer, inform it so that it can redial.
	if d := p.d; d != nil {
		go d.pipeClosed()
	}

	c.Lock()
	c.checkLimit(false)
	c.Unlock()
}

func (c *connector) SetNegotiator(negotiator Negotiator) {
	c.Lock()
	c.negotiator = negotiator
	c.Unlock()
}

func (c *connector) Dial(addr string) error {
	return c.DialOptions(addr, nil)
}

func (c *connector) DialOptions(addr string, ovs options.OptionValues) error {
	d, err := c.NewDialer(addr, ovs)
	if err != nil {
		return err
	}
	return d.Dial()
}

func (c *connector) NewDialer(addr string, ovs options.OptionValues) (d Dialer, err error) {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		err = errs.ErrClosed
		return
	}

	var (
		t  transport.Transport
		td transport.Dialer
	)

	if t = transport.GetTransportFromAddr(addr); t == nil {
		err = errs.ErrBadTransport
		return
	}

	if td, err = t.NewDialer(addr); err != nil {
		return
	}

	xd := newDialer(c, addr, td, ovs)
	if c.limit != -1 && c.limit <= len(c.pipes) {
		// exceed limit
		xd.stop()
	}
	d = xd

	c.dialers[xd] = struct{}{}
	return d, nil
}

func (c *connector) remDialer(d *dialer) {
	c.Lock()
	delete(c.dialers, d)
	c.Unlock()
	d.Close()
}

func (c *connector) StopDial(addr string) {
	// NOTE: keep connected pipes
	c.Lock()
	for d := range c.dialers {
		if d.addr == addr {
			delete(c.dialers, d)
			d.Close()
		}
	}
	c.Unlock()
}

func (c *connector) Listen(addr string) error {
	return c.ListenOptions(addr, nil)
}

func (c *connector) ListenOptions(addr string, ovs options.OptionValues) error {
	l, err := c.NewListener(addr, ovs)
	if err != nil {
		return err
	}
	return l.Listen()
}

func (c *connector) NewListener(addr string, ovs options.OptionValues) (l Listener, err error) {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		err = errs.ErrClosed
		return
	}

	var (
		t  transport.Transport
		tl transport.Listener
	)

	if t = transport.GetTransportFromAddr(addr); t == nil {
		err = errs.ErrBadTransport
		return
	}

	if tl, err = t.NewListener(addr); err != nil {
		return
	}

	xl := newListener(c, addr, tl, ovs)
	if c.limit != -1 && c.limit <= len(c.pipes) {
		// exceed limit
		xl.stop()
	}
	l = xl

	c.listeners[xl] = struct{}{}

	return
}

func (c *connector) StopListen(addr string) {
	// NOTE: keep accepted pipes
	c.Lock()
	for l := range c.listeners {
		if l.addr == addr {
			delete(c.listeners, l)
			l.Close()
		}
	}
	c.Unlock()
}

func (c *connector) GetPipe(id uint32) Pipe {
	c.Lock()
	p := c.pipes[id]
	c.Unlock()
	if p == nil {
		return nil
	}
	return p
}

func (c *connector) ClosePipe(id uint32) {
	p := c.GetPipe(id)
	if p != nil {
		p.Close()
	}
}

func (c *connector) Close() {
	c.Lock()
	if c.closed {
		c.Unlock()
		return
	}
	c.closed = true
	listeners := c.listeners
	dialers := c.dialers
	pipes := c.pipes

	c.listeners = nil
	c.dialers = nil
	c.pipes = nil
	c.Unlock()

	for l := range listeners {
		l.Close()
	}
	for d := range dialers {
		d.Close()
	}

	for _, p := range pipes {
		p.Close()
	}
}

func (c *connector) SetPipeEventHandler(handler PipeEventHandlerFunc) {
	c.Lock()
	c.pipeEventHandler = handler
	c.Unlock()
}

func (c *connector) ClearPipeEventHandler(handler PipeEventHandlerFunc) {
	c.Lock()
	c.pipeEventHandler = nil
	c.Unlock()
}
