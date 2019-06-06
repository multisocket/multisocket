package sender

import (
	"sync"

	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/socket"
)

type (
	sender struct {
		options.Options

		sync.Mutex
		attachedConnectors map[socket.Connector]struct{}
		closed             bool
		closedq            chan struct{}
		pipes              map[uint32]*pipe
		sendq              chan *socket.Message
	}

	pipe struct {
		p     socket.Pipe
		sendq chan *socket.Message
	}
)

const (
	defaultSendQueueSize = 128
)

func newPipe(p socket.Pipe) *pipe {
	return &pipe{
		p:     p,
		sendq: make(chan *socket.Message, defaultSendQueueSize),
	}
}

// New create a sender
func New() socket.Sender {
	return &sender{
		Options:            options.NewOptions(),
		attachedConnectors: make(map[socket.Connector]struct{}),
		closed:             false,
		closedq:            make(chan struct{}),
		pipes:              make(map[uint32]*pipe),
		sendq:              make(chan *socket.Message, defaultSendQueueSize),
	}
}

func (s *sender) AttachConnector(connector socket.Connector) {
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
	delete(s.pipes, id)
}

func (s *sender) run(p *pipe) {
	var (
		err error
		msg *socket.Message
	)

SENDING:
	for {
		select {
		case <-s.closedq:
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

	p.sendq <- s.newMsg(src, content)

	return
}

func (s *sender) SendMsg(msg *socket.Message) (err error) {
	// send to one
	// FIXME: 0, 1, n, N
	s.sendq <- msg

	return
}

func (s *sender) Send(content []byte) error {
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
