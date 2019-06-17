package sender

import (
	"sync"
	"time"

	"github.com/webee/multisocket/errs"

	"github.com/webee/multisocket/message"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/options"
)

type (
	sender struct {
		options.Options
		sendq   chan *Message
		closedq chan struct{}

		sync.Mutex
		attachedConnectors map[Connector]struct{}
		pipes              map[uint32]*pipe
	}

	pipe struct {
		p       Pipe
		closedq chan struct{}
		sendq   chan *Message
	}
)

const (
	defaultMsgTTL        = message.DefaultMsgTTL
	defaultSendQueueSize = uint16(64)
)

var (
	nilQ <-chan time.Time
)

// New create a sender
func New() Sender {
	return NewWithOptions(nil)
}

// NewWithOptions create a sender with options
func NewWithOptions(ovs options.OptionValues) Sender {
	s := &sender{
		attachedConnectors: make(map[Connector]struct{}),
		closedq:            make(chan struct{}),
		pipes:              make(map[uint32]*pipe),
	}
	s.Options = options.NewOptions().SetOptionChangeHook(s.onOptionChange)
	for opt, val := range ovs {
		s.SetOption(opt, val)
	}
	// default
	s.Options.SetOptionIfNotExists(OptionSendQueueSize, defaultSendQueueSize)
	return s
}

func (s *sender) onOptionChange(opt options.Option, oldVal, newVal interface{}) {
	switch opt {
	case OptionSendQueueSize:
		s.sendq = make(chan *Message, s.sendQueueSize())
	}
}

func (s *sender) doPushMsg(msg *Message, sendq chan<- *Message, closeq <-chan struct{}) (err error) {
	bestEffort := s.bestEffort()
	if bestEffort {
		select {
		case <-closeq:
			return errs.ErrClosed
		case <-s.closedq:
			return errs.ErrClosed
		case sendq <- msg:
			return nil
		default:
			// drop msg
			return ErrMsgDropped
		}
	}

	var timeoutTimer *time.Timer
	sendTimeout := s.sendTimeout()
	tq := nilQ
	if sendTimeout > 0 {
		timeoutTimer = time.NewTimer(sendTimeout)
		tq = timeoutTimer.C
	}

	select {
	case <-closeq:
		err = errs.ErrClosed
	case <-s.closedq:
		err = errs.ErrClosed
	case sendq <- msg:
	case <-tq:
		err = errs.ErrTimeout
	}
	if timeoutTimer != nil {
		timeoutTimer.Stop()
	}
	return
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

	connector.RegisterPipeEventHandler(s)
	s.attachedConnectors[connector] = struct{}{}
}

// options
func (s *sender) ttl() uint8 {
	return OptionTTL.Value(s.GetOptionDefault(OptionTTL, defaultMsgTTL))
}

func (s *sender) sendQueueSize() uint16 {
	return OptionSendQueueSize.Value(s.GetOptionDefault(OptionSendQueueSize, defaultSendQueueSize))
}

func (s *sender) bestEffort() bool {
	return OptionSendBestEffort.Value(s.GetOptionDefault(OptionSendBestEffort, false))
}

func (s *sender) sendTimeout() time.Duration {
	return OptionSendTimeout.Value(s.GetOptionDefault(OptionSendTimeout, time.Duration(0)))
}

func (s *sender) HandlePipeEvent(e PipeEvent, pipe Pipe) {
	switch e {
	case PipeEventAdd:
		s.addPipe(pipe)
	case PipeEventRemove:
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
		p.close()
	}
}

func (p *pipe) close() {
	select {
	case <-p.closedq:
	default:
		close(p.closedq)
		p.p.Close()
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
	if msg.Header.SendType() == SendTypeToOne {
		// only resend when send one, so we can choose another pipe to send.
		s.SendMsg(msg)
	}
}

func (s *sender) run(p *pipe) {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "sender").
			WithFields(log.Fields{"id": p.p.ID(), "raw": p.p.IsRaw()}).
			Debug("sender start run")
	}

	sendMsg := p.sendMsg
	if p.p.IsRaw() {
		sendMsg = p.sendRawMsg
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

		if err = sendMsg(msg); err != nil {
			s.resendMsg(msg)
			break SENDING
		}
	}
	s.remPipe(p.p.ID())
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "sender").
			WithFields(log.Fields{"id": p.p.ID(), "raw": p.p.IsRaw()}).
			Debug("sender stopped run")
	}
}

func (p *pipe) sendMsg(msg *Message) error {
	return p.p.Send(nil, msg.Encode()...)
}

func (p *pipe) sendRawMsg(msg *Message) (err error) {
	if msg.Header.HasAnyFlags() {
		// ignore none normal messages.
		return
	}
	return p.p.Send(msg.Content, msg.Extras...)
}

func (s *sender) newMsg(sendType uint8, dest MsgPath, content []byte, extras [][]byte) (msg *Message) {
	return newMessage(sendType, s.ttl(), dest, content, extras)
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
		err = ErrBrokenPath
		return
	}

	return s.doPushMsg(msg, p.sendq, p.closedq)
}

func (s *sender) SendTo(dest MsgPath, content []byte, extras ...[]byte) (err error) {
	return s.sendTo(s.newMsg(SendTypeToDest, dest, content, extras))
}

func (s *sender) Send(content []byte, extras ...[]byte) (err error) {
	return s.SendMsg(s.newMsg(SendTypeToOne, nil, content, extras))
}

func (s *sender) SendAll(content []byte, extras ...[]byte) (err error) {
	return s.SendMsg(s.newMsg(SendTypeToAll, nil, content, extras))
}

func (s *sender) SendMsg(msg *Message) error {
	switch msg.Header.SendType() {
	case SendTypeToDest:
		return s.sendTo(msg)
	case SendTypeToOne:
		return s.doPushMsg(msg, s.sendq, nil)
	case SendTypeToAll:
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
	select {
	case <-s.closedq:
		return
	default:
	}

	s.Lock()
	defer s.Unlock()

	// unregister
	for conns := range s.attachedConnectors {
		conns.UnregisterPipeEventHandler(s)
		delete(s.attachedConnectors, conns)
	}

	close(s.closedq)
}
