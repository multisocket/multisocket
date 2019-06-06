package receiver

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/socket"
)

type (
	receiver struct {
		options.Options

		recvNone bool // no recving

		sync.Mutex
		attachedConnectors map[socket.Connector]struct{}
		closed             bool
		closedq            chan struct{}
		pipes              map[uint32]*pipe
		recvq              chan *socket.Message
	}

	pipe struct {
		closedq chan struct{}
		p       socket.Pipe
	}
)

const (
	defaultRecvQueueSize = uint16(8)
)

func newPipe(p socket.Pipe) *pipe {
	return &pipe{
		closedq: make(chan struct{}),
		p:       p,
	}
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

// New create a normal receiver.
func New() socket.Receiver {
	return newReceiver(false)
}

// NewRecvNone create an recv none receiver.
func NewRecvNone() socket.Receiver {
	return newReceiver(true)
}

func newReceiver(recvNone bool) socket.Receiver {
	return &receiver{
		Options:            options.NewOptions(),
		recvNone:           recvNone,
		attachedConnectors: make(map[socket.Connector]struct{}),
		closed:             false,
		closedq:            make(chan struct{}),
		pipes:              make(map[uint32]*pipe),
	}
}

func (r *receiver) AttachConnector(connector socket.Connector) {
	// OptionRecvQueueSize useless after first attach
	if !r.recvNone {
		r.recvq = make(chan *socket.Message, OptionRecvQueueSize.Value(r.GetOptionDefault(OptionRecvQueueSize, defaultRecvQueueSize)))
	}
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
	p, ok := r.pipes[id]
	if ok {
		delete(r.pipes, id)
		// async close pipe, avoid dead lock the receiver.
		go r.closePipe(p)
	}
}

func (r *receiver) closePipe(p *pipe) {
	select {
	case <-p.closedq:
	default:
		close(p.closedq)
	}
}

func (r *receiver) run(p *pipe) {
	log.WithField("domain", "receiver").
		WithFields(log.Fields{"id": p.p.ID()}).
		Debug("pipe start run")
	var (
		err error
		msg *socket.Message
	)

RECVING:
	for {
		if msg, err = p.recvMsg(); err != nil {
			break
		}
		if !r.recvNone {
			r.recvq <- msg
		}
		// else drop msg

		select {
		case <-r.closedq:
			break RECVING
		case <-p.closedq:
			break RECVING
		default:
		}
	}
	r.remPipe(p.p.ID())
	log.WithField("domain", "receiver").
		WithFields(log.Fields{"id": p.p.ID()}).
		Debug("pipe stopped run")
}

func (r *receiver) RecvMsg() (msg *socket.Message, err error) {
	if r.recvNone {
		return
	}

	msg = <-r.recvq
	return
}

func (r *receiver) Recv() (content []byte, err error) {
	if r.recvNone {
		return
	}

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
