package connector

import (
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/webee/multisocket"
	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/transport"
)

type (
	connector struct {
		options.Options
		sync.Mutex
		limit             int
		dialers           []*dialer
		listeners         []*listener
		pipes             map[uint32]*pipe
		pipeEventHandlers map[multisocket.PipeEventHandler]struct{}
		closed            bool
	}
)

const (
	// defaultMaxRxMsgSize is the default maximum Rx Msg size
	defaultMaxRxMsgSize = 1024 * 1024
	// -1 no limit
	defaultConnLimit = -1
)

// New create a any Connector
func New() multisocket.Connector {
	return NewWithOptions()
}

// NewWithOptions create a Connector with options
func NewWithOptions(ovs ...*options.OptionValue) multisocket.Connector {
	return NewWithLimitAndOptions(defaultConnLimit, ovs...)
}

// NewWithLimitAndOptions create a Connector with limit and options
func NewWithLimitAndOptions(limit int, ovs ...*options.OptionValue) multisocket.Connector {
	c := &connector{
		limit:             limit,
		dialers:           make([]*dialer, 0),
		listeners:         make([]*listener, 0),
		pipes:             make(map[uint32]*pipe),
		pipeEventHandlers: make(map[multisocket.PipeEventHandler]struct{}),
	}
	c.Options = options.NewOptionsWithAccepts(OptionConnLimit, PipeOptionSendDeadline, PipeOptionRecvDeadline).SetOptionChangeHook(c.onOptionChange)
	for _, ov := range ovs {
		c.SetOption(ov.Option, ov.Value)
	}
	log.WithField("domain", "connector").
		WithField("limit", c.limit).
		Debug("create")
	return c
}

func (c *connector) onOptionChange(opt options.Option, oldVal, newVal interface{}) {
	switch opt {
	case OptionConnLimit:
		c.Lock()
		oldLimit := c.limit
		c.limit = OptionConnLimit.Value(newVal)
		log.WithField("domain", "connector").
			WithField("oldLimit", oldLimit).
			WithField("newLimit", c.limit).
			Debug("change limit")
		c.checkLimit(true)
		c.Unlock()
	}
}

// used by other functions, must get lock first
func (c *connector) checkLimit(checkNoLimit bool) {
	if checkNoLimit && c.limit == -1 {
		// start connecting
		for _, l := range c.listeners {
			l.start()
		}

		for _, d := range c.dialers {
			d.start()
		}
		log.WithField("domain", "connector").
			WithField("limit", c.limit).
			WithField("pipes", len(c.pipes)).
			WithField("action", "start").
			Debug("check limit")
	} else if c.limit != -1 && c.limit > len(c.pipes) {
		// below limit
		// start connecting
		for _, l := range c.listeners {
			l.start()
		}

		for _, d := range c.dialers {
			d.start()
		}
		log.WithField("domain", "connector").
			WithField("limit", c.limit).
			WithField("pipes", len(c.pipes)).
			WithField("action", "start").
			Debug("check limit")
	} else if c.limit != -1 && c.limit <= len(c.pipes) {
		// check exceed limit
		// stop connecting
		for _, l := range c.listeners {
			l.stop()
		}

		for _, d := range c.dialers {
			d.stop()
		}
		log.WithField("domain", "connector").
			WithField("limit", c.limit).
			WithField("pipes", len(c.pipes)).
			WithField("action", "stop").
			Debug("check limit")
	}
}

func (c *connector) addPipe(p *pipe) {
	c.Lock()
	defer c.Unlock()

	if c.limit == -1 || c.limit > len(c.pipes) {
		log.WithField("domain", "connector").
			WithFields(log.Fields{"id": p.ID(), "localAddress": p.LocalAddress(), "remoteAddress": p.RemoteAddress()}).
			WithField("limit", c.limit).
			WithField("pipes", len(c.pipes)).
			Debug("add pipe")

		c.pipes[p.ID()] = p
		for peh := range c.pipeEventHandlers {
			go peh.HandlePipeEvent(multisocket.PipeEventAdd, p)
		}

		c.checkLimit(false)
	} else {
		log.WithField("domain", "connector").
			WithFields(log.Fields{"id": p.ID(), "localAddress": p.LocalAddress(), "remoteAddress": p.RemoteAddress()}).
			WithField("limit", c.limit).
			WithField("pipes", len(c.pipes)).
			Debug("drop pipe")

		go p.Close()
	}
}

func (c *connector) remPipe(p *pipe) {
	c.Lock()
	delete(c.pipes, p.ID())
	for peh := range c.pipeEventHandlers {
		go peh.HandlePipeEvent(multisocket.PipeEventRemove, p)
	}
	c.Unlock()

	log.WithField("domain", "connector").
		WithFields(log.Fields{"id": p.ID()}).
		Debug("remove pipe")

	// If the pipe was from a dialer, inform it so that it can redial.
	if d := p.d; d != nil {
		go d.pipeClosed()
	}

	c.Lock()
	defer c.Unlock()
	c.checkLimit(false)
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

func (c *connector) NewDialer(addr string, opts options.Options) (d multisocket.Dialer, err error) {
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
		err = transport.ErrBadTran
		return
	}

	if td, err = t.NewDialer(addr); err != nil {
		return
	}
	td.SetOption(transport.OptionMaxRecvMsgSize, defaultMaxRxMsgSize)

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

func (c *connector) NewListener(addr string, opts options.Options) (l multisocket.Listener, err error) {
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
		err = transport.ErrBadTran
		return
	}

	if tl, err = t.NewListener(addr); err != nil {
		return
	}
	tl.SetOption(transport.OptionMaxRecvMsgSize, defaultMaxRxMsgSize)

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

func (c *connector) GetPipe(id uint32) (p multisocket.Pipe) {
	c.Lock()
	p = c.pipes[id]
	c.Unlock()
	return
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

func (c *connector) RegisterPipeEventHandler(handler multisocket.PipeEventHandler) {
	c.Lock()
	c.pipeEventHandlers[handler] = struct{}{}
	c.Unlock()
}

func (c *connector) UnregisterPipeEventHandler(handler multisocket.PipeEventHandler) {
	c.Lock()
	delete(c.pipeEventHandlers, handler)
	c.Unlock()
}
