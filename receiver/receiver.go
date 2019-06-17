package receiver

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/message"
	"github.com/webee/multisocket/options"
)

type (
	receiver struct {
		options.Options
		recvq   chan *Message
		closedq chan struct{}

		sync.Mutex
		attachedConnectors map[Connector]struct{}
		pipes              map[uint32]*pipe
	}

	pipe struct {
		p       Pipe
		closedq chan struct{}
	}
)

const (
	defaultRecvQueueSize = uint16(64)
)

var (
	nilQ    <-chan time.Time
	closedQ chan time.Time
)

func init() {
	closedQ = make(chan time.Time)
	close(closedQ)
}

// New create a receiver.
func New() Receiver {
	return NewWithOptions()
}

// NewWithOptions create a normal receiver with options.
func NewWithOptions(ovs ...*options.OptionValue) Receiver {
	r := &receiver{
		Options:            options.NewOptions(),
		attachedConnectors: make(map[Connector]struct{}),
		closedq:            make(chan struct{}),
		pipes:              make(map[uint32]*pipe),
	}
	for _, ov := range ovs {
		r.SetOption(ov.Option, ov.Value)
	}
	r.recvq = make(chan *Message, r.recvQueueSize())

	return r
}

func newPipe(p Pipe) *pipe {
	return &pipe{
		closedq: make(chan struct{}),
		p:       p,
	}
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

func (p *pipe) recvRawMsg() (msg *Message, err error) {
	var (
		payload []byte
	)
	if payload, err = p.p.Recv(); err != nil {
		return
	}

	msg = message.NewMessage(SendTypeToOne, nil, 0, payload)
	msg.Source = msg.Source.AddID(p.p.ID())
	msg.Header.Hops = msg.Source.Length()

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

func (r *receiver) recvTimeout() time.Duration {
	return OptionRecvTimeout.Value(r.GetOptionDefault(OptionRecvTimeout, time.Duration(0)))
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
	defer r.Unlock()
	p, ok := r.pipes[id]
	if ok {
		delete(r.pipes, id)
		p.close()
	}
}

func (p *pipe) close() {
	select {
	case <-p.closedq:
	default:
		close(p.closedq)
		p.p.Close()
	}
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
		// send a empty control to make a connection
		msg := message.NewMessage(SendTypeToOne, nil, message.MsgFlagControl, nil)
		// update source, add current pipe id
		msg.Source = msg.Source.AddID(p.p.ID())
		msg.Header.Hops = msg.Source.Length()

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
					WithFields(log.Fields{"id": p.p.ID()}).
					Debug("recvMsg")
			}
			break
		}
		if noRecv {
			// just drop
			continue
		}

		if msg == nil {
			// ignore nil msg
			continue
		}

		select {
		case <-r.closedq:
			break RECVING
		case <-p.closedq:
			// try recv pipe's last msg
			select {
			case <-r.closedq:
				break RECVING
			case r.recvq <- msg:
			}
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
	var (
		timeoutTimer *time.Timer
	)

	recvTimeout := r.recvTimeout()
	tq := nilQ
	if recvTimeout > 0 {
		timeoutTimer = time.NewTimer(recvTimeout)
		tq = timeoutTimer.C
	}

	select {
	case <-r.closedq:
		err = ErrClosed
	case <-tq:
		err = ErrTimeout
	case msg = <-r.recvq:
	}
	if timeoutTimer != nil {
		timeoutTimer.Stop()
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
	select {
	case <-r.closedq:
		return
	default:
	}

	r.Lock()
	defer r.Unlock()

	// unregister
	for conns := range r.attachedConnectors {
		conns.UnregisterPipeEventHandler(r)
		delete(r.attachedConnectors, conns)
	}

	close(r.closedq)
}
