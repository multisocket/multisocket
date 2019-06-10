package sender

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket"
	"github.com/webee/multisocket/options"
)

type (
	sender struct {
		options.Options

		sync.Mutex
		attachedConnectors map[Connector]struct{}
		closed             bool
		closedq            chan struct{}
		pipes              map[uint32]*pipe
		sendq              chan *Message
	}

	pipe struct {
		closedq chan struct{}
		p       Pipe
		sendq   chan *Message
	}
)

// Type aliases
type (
	// Message is message
	Message = multisocket.Message
	// Connector is connector
	Connector = multisocket.Connector
	// Sender is sender
	Sender = multisocket.Sender
	// Pipe is pipe
	Pipe = multisocket.Pipe
)

const (
	defaultMsgTTL        = uint8(16)
	defaultSendQueueSize = uint16(8)
)

var (
	nilQ <-chan time.Time
)

// New create a sender
func New() Sender {
	return NewWithOptions()
}

// NewWithOptions create a sender with options
func NewWithOptions(ovs ...*options.OptionValue) Sender {
	s := &sender{
		Options:            options.NewOptions(),
		attachedConnectors: make(map[Connector]struct{}),
		closed:             false,
		closedq:            make(chan struct{}),
		pipes:              make(map[uint32]*pipe),
	}
	for _, ov := range ovs {
		s.SetOption(ov.Option, ov.Value)
	}
	return s
}

func (s *sender) doPushMsg(msg *Message, sendq chan<- *Message, closeq <-chan struct{}) error {
	bestEffort := s.bestEffort()
	if bestEffort {
		select {
		case <-closeq:
			return ErrClosed
		case <-s.closedq:
			return ErrClosed
		case sendq <- msg:
			return nil
		default:
			// drop msg
			return nil
		}
	}

	sendDeadline := s.sendDeadline()
	tq := nilQ
	if sendDeadline > 0 {
		tq = time.After(sendDeadline)
	}

	select {
	case <-closeq:
		return ErrClosed
	case <-s.closedq:
		return ErrClosed
	case sendq <- msg:
		return nil
	case <-tq:
		return ErrTimeout
	}
}

func (s *sender) pushMsgToPipes(msg *Message, pipes []*pipe) {
	for _, p := range pipes {
		s.doPushMsg(msg, p.sendq, p.closedq)
	}
}

func (s *sender) newPipe(p Pipe) *pipe {
	return &pipe{
		closedq: make(chan struct{}),
		p:       p,
		sendq:   make(chan *Message, s.sendQueueSize()),
	}
}

func (s *sender) AttachConnector(connector Connector) {
	s.Lock()
	defer s.Unlock()

	// OptionSendQueueSize useless after first attach
	if s.sendq == nil {
		s.sendq = make(chan *Message, s.sendQueueSize())
	}

	connector.RegisterPipeEventHandler(s)
	s.attachedConnectors[connector] = struct{}{}
}

// options
func (s *sender) sendQueueSize() uint16 {
	return OptionSendQueueSize.Value(s.GetOptionDefault(OptionSendQueueSize, defaultSendQueueSize))
}

func (s *sender) bestEffort() bool {
	return OptionSendBestEffort.Value(s.GetOptionDefault(OptionSendBestEffort, false))
}

func (s *sender) sendDeadline() time.Duration {
	return OptionSendDeadline.Value(s.GetOptionDefault(OptionSendDeadline, time.Duration(0)))
}

func (s *sender) HandlePipeEvent(e multisocket.PipeEvent, pipe Pipe) {
	switch e {
	case multisocket.PipeEventAdd:
		s.addPipe(pipe)
	case multisocket.PipeEventRemove:
		s.remPipe(pipe.ID())
	}
}

func (s *sender) addPipe(pipe Pipe) {
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

func (s *sender) resendMsg(msg *Message) {
	if msg.Header.SendType == multisocket.SendTypeToOne {
		// only resend when send one, so we can choose another pipe to send.
		s.SendMsg(msg)
	}
}

func (s *sender) run(p *pipe) {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "sender").
			WithFields(log.Fields{"id": p.p.ID()}).
			Debug("pipe start run")
	}

	var (
		err error
		msg *Message
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

		if err = p.p.Send(msg.Encode()...); err != nil {
			s.resendMsg(msg)
			break SENDING
		}
	}
	s.remPipe(p.p.ID())
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "sender").
			WithFields(log.Fields{"id": p.p.ID()}).
			Debug("pipe stopped run")
	}
}

func (s *sender) newMsg(sendType uint8, dest multisocket.MsgPath, content []byte, extras [][]byte) (msg *Message) {
	ttl := OptionTTL.Value(s.GetOptionDefault(OptionTTL, defaultMsgTTL))
	return newMessage(sendType, ttl, dest, content, extras)
}

func (s *sender) sendTo(msg *Message) (err error) {
	var (
		id uint32
		ok bool
		p  *pipe
	)
	if msg.Header.Distance == 0 {
		// already arrived, just drop
		return
	}

	if id, ok = msg.Destination.CurID(); !ok {
		err = ErrBadDestination
		return
	}

	s.Lock()
	p = s.pipes[id]
	s.Unlock()
	if p == nil {
		err = ErrPipeNotFound
		return
	}

	return s.doPushMsg(msg, p.sendq, p.closedq)
}

func (s *sender) SendTo(dest multisocket.MsgPath, content []byte, extras ...[]byte) (err error) {
	return s.sendTo(s.newMsg(multisocket.SendTypeReply, dest, content, extras))
}

func (s *sender) Send(content []byte, extras ...[]byte) (err error) {
	return s.SendMsg(s.newMsg(multisocket.SendTypeToOne, nil, content, extras))
}

func (s *sender) SendAll(content []byte, extras ...[]byte) (err error) {
	return s.SendMsg(s.newMsg(multisocket.SendTypeToAll, nil, content, extras))
}

func (s *sender) SendMsg(msg *Message) error {
	switch msg.Header.SendType {
	case multisocket.SendTypeReply:
		return s.sendTo(msg)
	case multisocket.SendTypeToOne:
		return s.doPushMsg(msg, s.sendq, nil)
	case multisocket.SendTypeToAll:
		s.Lock()
		pipes := make([]*pipe, len(s.pipes))
		i := 0
		for _, p := range s.pipes {
			pipes[i] = p
			i++
		}
		s.Unlock()
		go s.pushMsgToPipes(msg, pipes)
		return nil
	}
	return ErrInvalidSendType
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
