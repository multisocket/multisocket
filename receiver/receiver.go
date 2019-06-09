package receiver

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket"
	"github.com/webee/multisocket/options"
)

type (
	receiver struct {
		options.Options

		recvNone bool // no recving

		sync.Mutex
		attachedConnectors map[multisocket.Connector]struct{}
		closed             bool
		closedq            chan struct{}
		pipes              map[uint32]*pipe
		recvq              chan *multisocket.Message
	}

	pipe struct {
		closedq chan struct{}
		p       multisocket.Pipe
	}
)

const (
	defaultRecvQueueSize = uint16(8)
)

func newPipe(p multisocket.Pipe) *pipe {
	return &pipe{
		closedq: make(chan struct{}),
		p:       p,
	}
}

func (p *pipe) recvMsg() (msg *multisocket.Message, err error) {
	var (
		payload []byte
		header  *multisocket.MsgHeader
		source  multisocket.MsgPath
		dest    multisocket.MsgPath
	)
	if payload, err = p.p.Recv(); err != nil {
		return
	}

	if header, err = NewHeaderFromBytes(payload); err != nil {
		return
	}
	headerSize := header.Size()
	source = NewPathFromBytes(int(header.Hops), payload[headerSize:])
	sourceSize := source.Size()
	dest = NewPathFromBytes(header.DestLength(), payload[headerSize+sourceSize:])
	content := payload[headerSize+sourceSize+dest.Size():]

	source = source.NewID(p.p.ID())
	header.Hops++
	header.TTL--
	msg = &multisocket.Message{
		Header:      header,
		Source:      source,
		Destination: dest,
		Content:     content,
	}
	return
}

// New create a normal receiver.
func New() multisocket.Receiver {
	return NewWithOptions()
}

// NewWithOptions create a normal receiver with options.
func NewWithOptions(ovs ...*options.OptionValue) multisocket.Receiver {
	return newWithOptions(false, ovs...)
}

// NewRecvNone create an recv none receiver.
func NewRecvNone() multisocket.Receiver {
	return newWithOptions(true)
}

func newWithOptions(recvNone bool, ovs ...*options.OptionValue) multisocket.Receiver {
	r := &receiver{
		Options:            options.NewOptions(),
		recvNone:           recvNone,
		attachedConnectors: make(map[multisocket.Connector]struct{}),
		closed:             false,
		closedq:            make(chan struct{}),
		pipes:              make(map[uint32]*pipe),
	}
	for _, ov := range ovs {
		r.SetOption(ov.Option, ov.Value)
	}

	return r
}

func (r *receiver) AttachConnector(connector multisocket.Connector) {
	r.Lock()
	defer r.Unlock()

	// OptionRecvQueueSize useless after first attach
	if !r.recvNone && r.recvq == nil {
		r.recvq = make(chan *multisocket.Message, r.recvQueueSize())
	}

	connector.RegisterPipeEventHandler(r)
	r.attachedConnectors[connector] = struct{}{}
}

// options
func (r *receiver) recvQueueSize() uint16 {
	return OptionRecvQueueSize.Value(r.GetOptionDefault(OptionRecvQueueSize, defaultRecvQueueSize))
}

func (r *receiver) HandlePipeEvent(e multisocket.PipeEvent, pipe multisocket.Pipe) {
	switch e {
	case multisocket.PipeEventAdd:
		r.addPipe(pipe)
	case multisocket.PipeEventRemove:
		r.remPipe(pipe.ID())
	}
}

func (r *receiver) addPipe(pipe multisocket.Pipe) {
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
		msg *multisocket.Message
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

func (r *receiver) RecvMsg() (msg *multisocket.Message, err error) {
	if r.recvNone {
		err = ErrOperationNotSupported
		return
	}

	msg = <-r.recvq
	return
}

func (r *receiver) Recv() (content []byte, err error) {
	if r.recvNone {
		err = ErrOperationNotSupported
		return
	}

	var msg *multisocket.Message
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
		conns.UnregisterPipeEventHandler(r)
		delete(r.attachedConnectors, conns)
	}

	close(r.closedq)
}
