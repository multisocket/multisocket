package receiver

import (
	"sync"

	"github.com/multisocket/multisocket/connector"

	"github.com/multisocket/multisocket/errs"

	log "github.com/sirupsen/logrus"
	"github.com/multisocket/multisocket/message"
	"github.com/multisocket/multisocket/options"
)

type (
	receiver struct {
		options.Options
		recvq chan *message.Message

		sync.Mutex
		closedq            chan struct{}
		attachedConnectors map[connector.Connector]struct{}
		pipes              map[uint32]*pipe
	}

	pipe struct {
		connector.Pipe
	}
)

var (
	emptyByteSlice = make([]byte, 0)
)

// New create a receiver.
func New() Receiver {
	return NewWithOptions(nil)
}

// NewWithOptions create a normal receiver with options.
func NewWithOptions(ovs options.OptionValues) Receiver {
	r := &receiver{
		attachedConnectors: make(map[connector.Connector]struct{}),
		closedq:            make(chan struct{}),
		pipes:              make(map[uint32]*pipe),
	}
	r.Options = options.NewOptions().SetOptionChangeHook(r.onOptionChange)
	for opt, val := range ovs {
		r.SetOption(opt, val)
	}
	// default
	r.onOptionChange(Options.RecvQueueSize, nil, nil)

	return r
}

func (r *receiver) onOptionChange(opt options.Option, oldVal, newVal interface{}) {
	switch opt {
	case Options.RecvQueueSize:
		r.recvq = make(chan *message.Message, r.recvQueueSize())
	}
}

func newPipe(cp connector.Pipe) *pipe {
	return &pipe{Pipe: cp}
}

func (r *receiver) AttachConnector(connector connector.Connector) {
	r.Lock()
	defer r.Unlock()

	connector.RegisterPipeEventHandler(r)
	r.attachedConnectors[connector] = struct{}{}
}

// options
func (r *receiver) recvQueueSize() uint16 {
	return r.GetOptionDefault(Options.RecvQueueSize).(uint16)
}

func (r *receiver) noRecv() bool {
	return r.GetOptionDefault(Options.NoRecv).(bool)
}

func (r *receiver) HandlePipeEvent(e connector.PipeEvent, pipe connector.Pipe) {
	switch e {
	case connector.PipeEventAdd:
		r.addPipe(pipe)
	case connector.PipeEventRemove:
		r.remPipe(pipe.ID())
	}
}

func (r *receiver) addPipe(pipe connector.Pipe) {
	r.Lock()
	p := newPipe(pipe)
	r.pipes[p.ID()] = p
	go r.run(p)
	r.Unlock()
}

func (r *receiver) remPipe(id uint32) {
	r.Lock()
	delete(r.pipes, id)
	r.Unlock()
}

func (r *receiver) run(p *pipe) {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "receiver").
			WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw()}).
			Debug("receiver start run")
	}

	var (
		noRecv = r.noRecv()
		err    error
		msg    *message.Message
	)

	if p.IsRaw() {
		// NOTE:
		// send a empty message to make a connection
		r.recvq <- message.NewRawRecvMessage(p.ID(), emptyByteSlice)
	}
RECVING:
	for {
		if msg, err = p.RecvMsg(); err != nil {
			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithField("domain", "receiver").
					WithError(err).
					WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw()}).
					Error("recvMsg")
			}
			if msg != nil {
				select {
				case <-r.closedq:
					msg.FreeAll()
					break RECVING
				case r.recvq <- msg:
				}
			}
			break RECVING
		}
		if msg == nil {
			// ignore nil msg
			continue
		}

		if noRecv {
			// just drop
			msg.FreeAll()
			continue
		}

		if msg.Header.HasFlags(message.MsgFlagInternal) {
			// FIXME: handle internal messages.
			msg.FreeAll()
			continue
		}

		select {
		case <-r.closedq:
			msg.FreeAll()
			break RECVING
		case r.recvq <- msg:
		}
	}

	r.remPipe(p.ID())
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "receiver").
			WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw()}).
			Debug("receiver stopped run")
	}
}

func (r *receiver) RecvMsg() (msg *message.Message, err error) {
	select {
	case <-r.closedq:
		// exhaust received messages
		select {
		case msg = <-r.recvq:
		default:
			err = errs.ErrClosed
		}
	case msg = <-r.recvq:
	}
	return
}

func (r *receiver) Recv() (content []byte, err error) {
	var msg *message.Message
	if msg, err = r.RecvMsg(); err != nil {
		return
	}
	return msg.ToContent(), nil
}

func (r *receiver) Close() {
	r.Lock()
	select {
	case <-r.closedq:
		r.Unlock()
		return
	default:
		close(r.closedq)
	}
	connectors := make([]connector.Connector, 0, len(r.attachedConnectors))
	for conns := range r.attachedConnectors {
		delete(r.attachedConnectors, conns)
		connectors = append(connectors, conns)
	}
	r.Unlock()

	// unregister
	for _, conns := range connectors {
		conns.UnregisterPipeEventHandler(r)
	}
}
