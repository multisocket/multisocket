package sender

import (
	"sync"

	"github.com/webee/multisocket/errs"

	"github.com/webee/multisocket/message"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/options"
)

type (
	sender struct {
		options.Options
		sendq chan *Message

		sync.Mutex
		closedq            chan struct{}
		attachedConnectors map[Connector]struct{}
		pipes              map[uint32]*pipe
	}

	pipe struct {
		stopq chan struct{}
		p     Pipe
		sendq chan *Message
	}
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
	s.onOptionChange(Options.SendQueueSize, nil, nil)
	return s
}

func (s *sender) onOptionChange(opt options.Option, oldVal, newVal interface{}) {
	switch opt {
	case Options.SendQueueSize:
		s.sendq = make(chan *Message, s.sendQueueSize())
	}
}

func (s *sender) doPushMsg(msg *Message, sendq chan<- *Message) (err error) {
	bestEffort := s.bestEffort()
	if bestEffort {
		select {
		case <-s.closedq:
			return errs.ErrClosed
		case sendq <- msg:
			return nil
		default:
			// drop msg
			return ErrMsgDropped
		}
	}

	select {
	case <-s.closedq:
		err = errs.ErrClosed
	case sendq <- msg:
	}
	return
}

func (s *sender) pushMsgToPipes(msg *Message, pipes []*pipe) {
	for _, p := range pipes {
		s.doPushMsg(msg, p.sendq)
	}
}

func (s *sender) newPipe(p Pipe) *pipe {
	return &pipe{
		stopq: make(chan struct{}),
		p:     p,
		sendq: make(chan *Message, s.sendQueueSize()),
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
	return Options.TTL.ValueFrom(s.Options)
}

func (s *sender) sendQueueSize() uint16 {
	return Options.SendQueueSize.ValueFrom(s.Options)
}

func (s *sender) bestEffort() bool {
	return Options.SendBestEffort.ValueFrom(s.Options)
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
	p, ok := s.pipes[id]
	if !ok {
		s.Unlock()
		return
	}
	delete(s.pipes, id)
	s.Unlock()

	p.stop()
}

func (p *pipe) stop() {
	close(p.stopq)
DRAIN_MSG_LOOP:
	for {
		select {
		case <-p.sendq:
			// send to dest/all msgs, just drop
			// TODO: maybe free msg bytes.
		default:
			break DRAIN_MSG_LOOP
		}
	}
}

func (s *sender) resendMsg(msg *Message) bool {
	if msg.Header.SendType() == SendTypeToOne {
		// only resend when send one, so we can choose another pipe to send.
		if err := s.SendMsg(msg); err == nil {
			return true
		}
	}
	return false
}

func (s *sender) run(p *pipe) {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "sender").
			WithFields(log.Fields{"id": p.p.ID(), "raw": p.p.IsRaw()}).
			Debug("sender start run")
	}

	sendMsg := p.sendMsg
	sendq := s.sendq
	if p.p.IsRaw() {
		sendMsg = p.sendRawMsg
		// raw pipe should not recv send to one messages.
		sendq = nil
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
		case <-p.stopq:
			break SENDING
		case msg = <-p.sendq:
		case msg = <-sendq:
		}
		if msg.Header.TTL == 0 {
			// drop msg
			continue
		}

		if err = sendMsg(msg); err != nil {
			if !s.resendMsg(msg) {
				// TODO: maybe free msg bytes.
			}

			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithField("domain", "sender").
					WithError(err).
					WithFields(log.Fields{"id": p.p.ID(), "raw": p.p.IsRaw()}).
					Error("sendMsg")
			}
			break SENDING
		}
	}
	// seems can be moved to case <-s.closedq
	s.remPipe(p.p.ID())
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "sender").
			WithFields(log.Fields{"id": p.p.ID(), "raw": p.p.IsRaw()}).
			Debug("sender stopped run")
	}
}

func (p *pipe) sendMsg(msg *Message) error {
	if msg.Header.HasFlags(message.MsgFlagRaw) {
		// ignore raw messages. raw message is only for stream, forward raw message makes no sense.
		return nil
	}
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

	return s.doPushMsg(msg, p.sendq)
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
		return s.doPushMsg(msg, s.sendq)
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
	s.Lock()
	select {
	case <-s.closedq:
		s.Unlock()
		return
	default:
		close(s.closedq)
	}
	connectors := make([]Connector, 0, len(s.attachedConnectors))
	for conns := range s.attachedConnectors {
		delete(s.attachedConnectors, conns)
		connectors = append(connectors, conns)
	}
	s.Unlock()

	// unregister
	for _, conns := range connectors {
		conns.UnregisterPipeEventHandler(s)
	}
}
