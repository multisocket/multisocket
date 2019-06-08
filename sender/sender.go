package sender

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket"
	"github.com/webee/multisocket/options"
)

type (
	sender struct {
		options.Options

		sendType     SendType
		pipeSelector PipeSelector

		sync.Mutex
		attachedConnectors map[multisocket.Connector]struct{}
		closed             bool
		closedq            chan struct{}
		pipes              map[uint32]*pipe
		sendq              chan *multisocket.Message
	}

	pipe struct {
		closedq chan struct{}
		p       multisocket.Pipe
		sendq   chan *multisocket.Message
	}

	// SendType 0, 1, n, N
	SendType int

	// PipeSelector is for selecting pipes to send.
	PipeSelector interface {
		Select(pipes map[uint32]*pipe) []*pipe
	}
)

// sender types
const (
	// random select one pipe to send
	SendOne SendType = iota
	// select some pipes to send
	SendSome
	// send to all pipes
	SendAll
)

const (
	defaultSendQueueSize = uint16(8)
)

func (p *pipe) pushMsg(msg *multisocket.Message) error {
	select {
	case <-p.closedq:
		return ErrClosed
	default:
		// TODO: add send timeout
		p.sendq <- msg
	}
	return nil
}

// New create a SendOne sender
func New() multisocket.Sender {
	return NewWithOptions(SendOne, nil)
}

// NewSendOneWithOptions create a SendOne sender with options
func NewSendOneWithOptions(ovs ...*options.OptionValue) multisocket.Sender {
	return NewWithOptions(SendOne, nil, ovs...)
}

// NewSendAll create a SendAll sender
func NewSendAll() multisocket.Sender {
	return NewWithOptions(SendAll, nil)
}

// NewSendAllWithOptions create a SendAll sender with options
func NewSendAllWithOptions(ovs ...*options.OptionValue) multisocket.Sender {
	return NewWithOptions(SendAll, nil, ovs...)
}

// NewSelectSend create a SendSome sender
func NewSelectSend(pipeSelector PipeSelector) multisocket.Sender {
	return NewWithOptions(SendSome, pipeSelector)
}

// NewSelectSendWithOptions create a SendSome sender with options
func NewSelectSendWithOptions(pipeSelector PipeSelector, ovs ...*options.OptionValue) multisocket.Sender {
	return NewWithOptions(SendSome, pipeSelector, ovs...)
}

// NewWithOptions create a sender with options
func NewWithOptions(sendType SendType, pipeSelector PipeSelector, ovs ...*options.OptionValue) multisocket.Sender {
	s := &sender{
		Options:            options.NewOptions(),
		sendType:           sendType,
		pipeSelector:       pipeSelector,
		attachedConnectors: make(map[multisocket.Connector]struct{}),
		closed:             false,
		closedq:            make(chan struct{}),
		pipes:              make(map[uint32]*pipe),
	}
	for _, ov := range ovs {
		s.SetOption(ov.Option, ov.Value)
	}
	return s
}

func (s *sender) newPipe(p multisocket.Pipe) *pipe {
	return &pipe{
		closedq: make(chan struct{}),
		p:       p,
		sendq:   make(chan *multisocket.Message, s.sendQueueSize()),
	}
}

func (s *sender) AttachConnector(connector multisocket.Connector) {
	s.Lock()
	defer s.Unlock()

	// OptionSendQueueSize useless after first attach
	if s.sendq == nil {
		s.sendq = make(chan *multisocket.Message, s.sendQueueSize())
	}

	connector.RegisterPipeEventHandler(s)
	s.attachedConnectors[connector] = struct{}{}
}

// options
func (s *sender) sendQueueSize() uint16 {
	return OptionSendQueueSize.Value(s.GetOptionDefault(OptionSendQueueSize, defaultSendQueueSize))
}

func (s *sender) HandlePipeEvent(e multisocket.PipeEvent, pipe multisocket.Pipe) {
	switch e {
	case multisocket.PipeEventAdd:
		s.addPipe(pipe)
	case multisocket.PipeEventRemove:
		s.remPipe(pipe.ID())
	}
}

func (s *sender) addPipe(pipe multisocket.Pipe) {
	s.Lock()
	defer s.Unlock()
	p := s.newPipe(pipe)
	s.pipes[p.p.ID()] = p
	go s.run(p)
}

func (s *sender) remPipe(id uint32) {
	s.Lock()
	defer s.Unlock()
	p, ok := s.pipes[id]
	if ok {
		delete(s.pipes, id)
		// async close pipe, avoid dead lock the sender.
		go s.closePipe(p)
	}
}

func (s *sender) closePipe(p *pipe) {
	select {
	case <-p.closedq:
	default:
		close(p.closedq)
	DROP_MSG_LOOP:
		for {
			select {
			case <-p.sendq:
				// send some/all msgs, just drop
				// TODO: maybe free msg bytes.
			default:
				break DROP_MSG_LOOP
			}
		}
	}
}

func (s *sender) resendMsg(msg *multisocket.Message) {
	if s.sendType == SendOne {
		// only resend when send one
		if msg.Source == nil || msg.Header.Hops > 0 {
			// re send, initiative/forward send msgs.
			s.SendMsg(msg)
		}
	}
}

func (s *sender) run(p *pipe) {
	log.WithField("domain", "sender").
		WithFields(log.Fields{"id": p.p.ID()}).
		Debug("pipe start run")

	var (
		err error
		msg *multisocket.Message
	)

SENDING:
	for {
		select {
		case <-s.closedq:
			break SENDING
		case <-p.closedq:
			break SENDING
		case msg = <-s.sendq:
		case msg = <-p.sendq:
		}
		if msg.Header.TTL == 0 {
			// drop msg
			continue
		}

		if err = p.p.Send(msg.Header.Encode(), msg.Source.Encode(), msg.Content); err != nil {
			s.resendMsg(msg)
			break SENDING
		}
	}
	s.remPipe(p.p.ID())
	log.WithField("domain", "sender").
		WithFields(log.Fields{"id": p.p.ID()}).
		Debug("pipe stopped run")
}

func (s *sender) newMsg(src multisocket.MsgSource, content []byte) (msg *multisocket.Message) {
	msg = NewMessage(src, content)
	if val, ok := s.GetOption(OptionTTL); ok {
		msg.Header.TTL = OptionTTL.Value(val)
	}
	return
}

func (s *sender) SendTo(src multisocket.MsgSource, content []byte) (err error) {
	var (
		id uint32
		ok bool
		p  *pipe
	)
	if id, src, ok = src.NextID(); !ok {
		return
	}

	s.Lock()
	p = s.pipes[id]
	s.Unlock()
	if p == nil {
		return
	}

	return p.pushMsg(s.newMsg(src, content))
}

func pushMsgToPipes(msg *multisocket.Message, pipes []*pipe) {
	for _, p := range pipes {
		p.pushMsg(msg)
	}
}

func (s *sender) SendMsg(msg *multisocket.Message) (err error) {
	switch s.sendType {
	case SendOne:
		// TODO: add send timeout
		s.sendq <- msg
	case SendAll:
		s.Lock()
		pipes := make([]*pipe, len(s.pipes))
		i := 0
		for _, p := range s.pipes {
			pipes[i] = p
			i++
		}
		s.Unlock()
		go pushMsgToPipes(msg, pipes)
	case SendSome:
		s.Lock()
		pipes := s.pipeSelector.Select(s.pipes)
		s.Unlock()
		go pushMsgToPipes(msg, pipes)
	}

	return
}

func (s *sender) Send(content []byte) (err error) {
	return s.SendMsg(s.newMsg(nil, content))
}

func (s *sender) Close() {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return
	}
	s.closed = true

	// unregister
	for conns := range s.attachedConnectors {
		conns.UnregisterPipeEventHandler(s)
		delete(s.attachedConnectors, conns)
	}

	close(s.closedq)
}
