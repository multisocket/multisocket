package receiver

import (
	"sync"

	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/socket"
)

type (
	receiver struct {
		options.Options

		sync.Mutex
		attachedConnectors map[socket.Connector]struct{}
		closed             bool
		closedq            chan struct{}
		pipes              map[uint32]*pipe
		recvq              chan *socket.Message
	}

	pipe struct {
		p socket.Pipe
	}
)

const (
	defaultRecvQueueSize = 256
)

func newPipe(p socket.Pipe) *pipe {
	return &pipe{p}
}

func (p *pipe) recvMsg() (msg *socket.Message, err error) {
	var (
		payload []byte
		header  *socket.MsgHeader
		source  socket.MsgSource
	)
	if payload, err = p.p.Recv(); err != nil {
		return
	}

	if header, err = socket.NewHeaderFromBytes(payload); err != nil {
		return
	}
	headerSize := header.Size()
	source = socket.NewSourceFromBytes(int(header.Hops), payload[headerSize:])
	content := payload[headerSize+source.Size():]

	source = source.NewID(p.p.ID())
	header.Hops++
	header.TTL--
	msg = &socket.Message{
		Header:  header,
		Source:  source,
		Content: content,
	}
	return
}

// New create a receiver
func New() socket.Receiver {
	return &receiver{
		Options:            options.NewOptions(),
		attachedConnectors: make(map[socket.Connector]struct{}),
		closed:             false,
		closedq:            make(chan struct{}),
		pipes:              make(map[uint32]*pipe),
		recvq:              make(chan *socket.Message, defaultRecvQueueSize),
	}
}

func (r *receiver) AttachConnector(connector socket.Connector) {
	r.Lock()
	defer r.Unlock()

	connector.RegisterPipeEventHook(r.handlePipeEvent)
	r.attachedConnectors[connector] = struct{}{}
}

func (r *receiver) handlePipeEvent(e socket.PipeEvent, pipe socket.Pipe) {
	switch e {
	case socket.PipeEventAdd:
		r.addPipe(pipe)
	case socket.PipeEventRemove:
		r.remPipe(pipe.ID())
	}
}

func (r *receiver) addPipe(pipe socket.Pipe) {
	r.Lock()
	defer r.Unlock()
	p := newPipe(pipe)
	r.pipes[p.p.ID()] = p
	go r.run(p)
}

func (r *receiver) remPipe(id uint32) {
	r.Lock()
	defer r.Unlock()
	delete(r.pipes, id)
}

func (r *receiver) run(p *pipe) {
	var (
		err error
		msg *socket.Message
	)

RECVING:
	for {
		if msg, err = p.recvMsg(); err != nil {
			break
		}
		select {
		case <-r.closedq:
			break RECVING
		case r.recvq <- msg:
		}
	}
	r.remPipe(p.p.ID())
}

func (r *receiver) RecvMsg() (msg *socket.Message, err error) {
	msg = <-r.recvq
	return
}

func (r *receiver) Recv() (content []byte, err error) {
	var msg *socket.Message
	if msg, err = r.RecvMsg(); err != nil {
		return
	}
	content = msg.Content
	return
}

func (r *receiver) Close() {
	r.Lock()
	defer r.Unlock()
	if r.closed {
		return
	}
	r.closed = true

	// unregister
	for conns := range r.attachedConnectors {
		conns.UnregisterPipeEventHook(r.handlePipeEvent)
		delete(r.attachedConnectors, conns)
	}

	close(r.closedq)
}
