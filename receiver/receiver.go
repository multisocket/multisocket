package receiver

import (
	"io"
	"sync"

	"github.com/webee/multisocket/errs"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/message"
	"github.com/webee/multisocket/options"
)

type (
	receiver struct {
		options.Options
		recvq chan *Message

		sync.Mutex
		closedq            chan struct{}
		attachedConnectors map[Connector]struct{}
		pipes              map[uint32]*pipe
	}

	pipe struct {
		p Pipe
	}
)

const (
	defaultRecvQueueSize = uint16(64)
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
		attachedConnectors: make(map[Connector]struct{}),
		closedq:            make(chan struct{}),
		pipes:              make(map[uint32]*pipe),
	}
	r.Options = options.NewOptions().SetOptionChangeHook(r.onOptionChange)
	for opt, val := range ovs {
		r.SetOption(opt, val)
	}
	// default
	r.Options.SetOptionIfNotExists(OptionRecvQueueSize, defaultRecvQueueSize)

	return r
}

func (r *receiver) onOptionChange(opt options.Option, oldVal, newVal interface{}) {
	switch opt {
	case OptionRecvQueueSize:
		r.recvq = make(chan *Message, r.recvQueueSize())
	}
}

func newPipe(p Pipe) *pipe {
	return &pipe{p}
}

func (p *pipe) recvMsg() (msg *Message, err error) {
	var (
		ok      bool
		payload []byte
		header  *MsgHeader
		source  MsgPath
		dest    MsgPath
	)
	if payload, err = p.p.Recv(); err != nil {
		return
	}

	if header, err = newHeaderFromBytes(payload); err != nil {
		return
	}
	sendType := header.SendType()
	headerSize := header.Size()

	source = newPathFromBytes(int(header.Hops), payload[headerSize:])
	sourceSize := source.Size()

	if sendType == SendTypeToDest {
		dest = newPathFromBytes(header.DestLength(), payload[headerSize+sourceSize:])
	}

	content := payload[headerSize+sourceSize+dest.Size():]

	// update source, add current pipe id
	source = source.AddID(p.p.ID())
	header.Hops++
	header.TTL--

	if sendType == SendTypeToDest {
		// update destination, remove last pipe id
		if _, dest, ok = dest.NextID(); !ok {
			// anyway, msg arrived
			header.Distance = 0
		} else {
			header.Distance = dest.Length()
		}
	}

	msg = &Message{
		Header:      header,
		Source:      source,
		Destination: dest,
		Content:     content,
	}
	return
}

func newRawMsg(pipeID uint32, content []byte) *message.Message {
	// raw message is always send to one.
	msg := message.NewMessage(SendTypeToOne, nil, message.MsgFlagRaw, content)
	// update source, add current pipe id
	msg.Source = msg.Source.AddID(pipeID)
	msg.Header.Hops = msg.Source.Length()
	return msg
}

func (p *pipe) recvRawMsg() (msg *Message, err error) {
	var (
		payload []byte
	)
	if payload, err = p.p.Recv(); err != nil {
		if err == io.EOF {
			// nil content as EOF signal
			msg = newRawMsg(p.p.ID(), nil)
		}
		return
	}

	msg = newRawMsg(p.p.ID(), payload)

	return
}

func (r *receiver) AttachConnector(connector Connector) {
	r.Lock()
	defer r.Unlock()

	connector.RegisterPipeEventHandler(r)
	r.attachedConnectors[connector] = struct{}{}
}

// options
func (r *receiver) recvQueueSize() uint16 {
	return OptionRecvQueueSize.Value(r.GetOptionDefault(OptionRecvQueueSize, defaultRecvQueueSize))
}

func (r *receiver) noRecv() bool {
	return OptionNoRecv.Value(r.GetOptionDefault(OptionNoRecv, false))
}

func (r *receiver) HandlePipeEvent(e PipeEvent, pipe Pipe) {
	switch e {
	case PipeEventAdd:
		r.addPipe(pipe)
	case PipeEventRemove:
		r.remPipe(pipe.ID())
	}
}

func (r *receiver) addPipe(pipe Pipe) {
	r.Lock()
	defer r.Unlock()
	p := newPipe(pipe)
	r.pipes[p.p.ID()] = p
	go r.run(p)
}

func (r *receiver) remPipe(id uint32) {
	r.Lock()
	delete(r.pipes, id)
	r.Unlock()
}

func (r *receiver) run(p *pipe) {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "receiver").
			WithFields(log.Fields{"id": p.p.ID(), "raw": p.p.IsRaw()}).
			Debug("receiver start run")
	}

	recvMsg := p.recvMsg
	if p.p.IsRaw() {
		recvMsg = p.recvRawMsg

		// NOTE:
		// send a empty message to make a connection
		msg := newRawMsg(p.p.ID(), emptyByteSlice)

		r.recvq <- msg
	}

	var (
		noRecv = r.noRecv()
		err    error
		msg    *Message
	)
RECVING:
	for {
		if msg, err = recvMsg(); err != nil {
			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithField("domain", "receiver").
					WithError(err).
					WithFields(log.Fields{"id": p.p.ID(), "raw": p.p.IsRaw()}).
					Error("recvMsg")
			}
			if msg != nil {
				select {
				case <-r.closedq:
					break RECVING
				case r.recvq <- msg:
				}
			}
			break RECVING
		}
		if noRecv || msg == nil {
			// just drop or ignore nil msg
			continue
		}

		if msg.Header.HasFlags(message.MsgFlagInternal) {
			// TODO: handle internal messages.
			continue
		}

		select {
		case <-r.closedq:
			break RECVING
		case r.recvq <- msg:
		}
	}

	r.remPipe(p.p.ID())
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "receiver").
			WithFields(log.Fields{"id": p.p.ID(), "raw": p.p.IsRaw()}).
			Debug("receiver stopped run")
	}
}

func (r *receiver) RecvMsg() (msg *Message, err error) {
	select {
	case <-r.closedq:
		err = errs.ErrClosed
	case msg = <-r.recvq:
	}
	return
}

func (r *receiver) Recv() (content []byte, err error) {
	var msg *Message
	if msg, err = r.RecvMsg(); err != nil {
		return
	}
	content = msg.Content
	return
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

	// unregister
	for conns := range r.attachedConnectors {
		conns.UnregisterPipeEventHandler(r)
		delete(r.attachedConnectors, conns)
	}

	r.Unlock()
}
