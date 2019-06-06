package sender

import (
	"sync"

	"github.com/webee/multisocket/socket"
)

type (
	sender struct {
		sync.Mutex
		attachedConnectors map[socket.Connector]struct{}
		closed             bool
		closedq            chan struct{}
		pipes              map[uint32]*pipe
		sendq              chan [][]byte
	}

	pipe struct {
		p     socket.Pipe
		sendq chan [][]byte
	}
)

const (
	defaultSendQueueSize = 128
)

func newPipe(p socket.Pipe) *pipe {
	return &pipe{
		p:     p,
		sendq: make(chan [][]byte, defaultSendQueueSize),
	}
}

// New create a sender
func New() socket.Sender {
	return &sender{
		attachedConnectors: make(map[socket.Connector]struct{}),
		closed:             false,
		closedq:            make(chan struct{}),
		pipes:              make(map[uint32]*pipe),
		sendq:              make(chan [][]byte, defaultSendQueueSize),
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
		err  error
		msgs [][]byte
	)

SENDING:
	for {
		select {
		case <-s.closedq:
			break SENDING
		case msgs = <-s.sendq:
		case msgs = <-p.sendq:
		}
		if err = p.p.Send(msgs...); err != nil {
			break SENDING
		}
	}
	s.remPipe(p.p.ID())
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

	msg := socket.NewMessage(src, content)
	p.sendq <- [][]byte{msg.Header.Encode(), msg.Source.Encode(), msg.Content}

	return
}

func (s *sender) SendMsg(msg *socket.Message) (err error) {
	// send to one
	// FIXME: 0, 1, n, N
	msgs := [][]byte{msg.Header.Encode(), msg.Source.Encode(), msg.Content}
	s.sendq <- msgs

	return
}

func (s *sender) Send(content []byte) error {
	return s.SendMsg(socket.NewMessage(nil, content))
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
