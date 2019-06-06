package sender

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/socket"
)

type (
	sender struct {
		options.Options

		sendType     SendType
		pipeSelector PipeSelector

		sync.Mutex
		attachedConnectors map[socket.Connector]struct{}
		closed             bool
		closedq            chan struct{}
		pipes              map[uint32]*pipe
		sendq              chan *socket.Message
	}

	pipe struct {
		closedq chan struct{}
		p       socket.Pipe
		sendq   chan *socket.Message
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

func newPipe(p socket.Pipe) *pipe {
	return &pipe{
		closedq: make(chan struct{}),
		p:       p,
		sendq:   make(chan *socket.Message, defaultSendQueueSize),
	}
}

func (p *pipe) pushMsg(msg *socket.Message) error {
	select {
	case <-p.closedq:
		return ErrPipeClosed
	default:
		// TODO: add send timeout
		p.sendq <- msg
	}
	return nil
}

// New create a SendOne sender
func New() socket.Sender {
	return newSender(SendOne, nil)
}

// NewSendAll create a SendAll sender
func NewSendAll() socket.Sender {
	return newSender(SendOne, nil)
}

// NewSelectSend create a SendSome sender
func NewSelectSend(pipeSelector PipeSelector) socket.Sender {
	return newSender(SendSome, pipeSelector)
}

func newSender(sendType SendType, pipeSelector PipeSelector) socket.Sender {
	return &sender{
		Options:            options.NewOptions(),
		sendType:           sendType,
		pipeSelector:       pipeSelector,
		attachedConnectors: make(map[socket.Connector]struct{}),
		closed:             false,
		closedq:            make(chan struct{}),
		pipes:              make(map[uint32]*pipe),
	}
}

func (s *sender) AttachConnector(connector socket.Connector) {
	// OptionSendQueueSize useless after first attach
	s.sendq = make(chan *socket.Message, OptionSendQueueSize.Value(s.GetOptionDefault(OptionSendQueueSize, defaultSendQueueSize)))

	s.Lock()
	defer s.Unlock()

	connector.RegisterPipeEventHook(s.handlePipeEvent)
	s.attachedConnectors[connector] = struct{}{}
}

func (s *sender) handlePipeEvent(e socket.PipeEvent, pipe socket.Pipe) {
	switch e {
	case socket.PipeEventAdd:
		s.addPipe(pipe)
	case socket.PipeEventRemove:
		s.remPipe(pipe.ID())
	}
}

func (s *sender) addPipe(pipe socket.Pipe) {
	s.Lock()
	defer s.Unlock()
	p := newPipe(pipe)
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
	DROPING_MSGS_LOOP:
		for {
			select {
			case msg := <-p.sendq:
				if s.sendType == SendOne {
					// only resend when send one
					if msg.Source == nil || msg.Header.Hops > 0 {
						// re send, initiative/forward send msgs.
						s.SendMsg(msg)
					}
				}
			default:
				break DROPING_MSGS_LOOP
			}
		}
	}
}

func (s *sender) run(p *pipe) {
	log.WithField("domain", "sender").
		WithFields(log.Fields{"id": p.p.ID()}).
		Debug("pipe start run")

	var (
		err error
		msg *socket.Message
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
			break SENDING
		}
	}
	s.remPipe(p.p.ID())
	log.WithField("domain", "sender").
		WithFields(log.Fields{"id": p.p.ID()}).
		Debug("pipe stopped run")
}

func (s *sender) newMsg(src socket.MsgSource, content []byte) (msg *socket.Message) {
	msg = socket.NewMessage(src, content)
	if val, ok := s.GetOption(OptionTTL); ok {
		msg.Header.TTL = OptionTTL.Value(val)
	}
	return
}

func (s *sender) SendTo(src socket.MsgSource, content []byte) (err error) {
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

func pushMsgToPipes(msg *socket.Message, pipes []*pipe) {
	for _, p := range pipes {
		p.pushMsg(msg)
	}
}

func (s *sender) SendMsg(msg *socket.Message) (err error) {
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
		conns.UnregisterPipeEventHook(s.handlePipeEvent)
		delete(s.attachedConnectors, conns)
	}

	close(s.closedq)
}
