package sender

import (
	"sync"

	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/errs"
	"github.com/webee/multisocket/message"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/options"
)

type (
	sender struct {
		options.Options
		sendq chan *message.Message

		sync.RWMutex
		closedq            chan struct{}
		attachedConnectors map[connector.Connector]struct{}
		pipes              map[uint32]*pipe
	}

	pipe struct {
		connector.Pipe
		stopq chan struct{}
		sendq chan *message.Message
	}
)

// New create a sender
func New() Sender {
	return NewWithOptions(nil)
}

// NewWithOptions create a sender with options
func NewWithOptions(ovs options.OptionValues) Sender {
	s := &sender{
		attachedConnectors: make(map[connector.Connector]struct{}),
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
		s.sendq = make(chan *message.Message, s.sendQueueSize())
	}
}

func (s *sender) doPushMsg(msg *message.Message, sendq chan<- *message.Message) (err error) {
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

func (s *sender) pushMsgToPipes(msg *message.Message, pipes []*pipe) {
	for _, p := range pipes {
		s.doPushMsg(msg, p.sendq)
	}
}

func (s *sender) newPipe(cp connector.Pipe) *pipe {
	return &pipe{
		Pipe:  cp,
		stopq: make(chan struct{}),
		sendq: make(chan *message.Message, s.sendQueueSize()),
	}
}

func (s *sender) AttachConnector(connector connector.Connector) {
	s.Lock()
	defer s.Unlock()

	connector.RegisterPipeEventHandler(s)
	s.attachedConnectors[connector] = struct{}{}
}

// options
func (s *sender) ttl() uint8 {
	return s.GetOptionDefault(Options.TTL).(uint8)
}

func (s *sender) sendQueueSize() uint16 {
	return s.GetOptionDefault(Options.SendQueueSize).(uint16)
}

func (s *sender) bestEffort() bool {
	return s.GetOptionDefault(Options.SendBestEffort).(bool)
}

func (s *sender) HandlePipeEvent(e connector.PipeEvent, pipe connector.Pipe) {
	switch e {
	case connector.PipeEventAdd:
		s.addPipe(pipe)
	case connector.PipeEventRemove:
		s.remPipe(pipe.ID())
	}
}

func (s *sender) addPipe(pipe connector.Pipe) {
	s.Lock()
	p := s.newPipe(pipe)
	s.pipes[p.ID()] = p
	go s.run(p)
	s.Unlock()
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

func (s *sender) resendMsg(msg *message.Message) error {
	if msg.Header.SendType() == message.SendTypeToOne {
		// only resend when send to one, so we can choose another pipe to send.
		return s.doPushMsg(msg, s.sendq)
	}
	return nil
}

func (s *sender) run(p *pipe) {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "sender").
			WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw()}).
			Debug("sender start run")
	}

	var (
		err error
		msg *message.Message
	)

	sendq := s.sendq
	if p.IsRaw() {
		// raw pipe should not recv send to one messages.
		sendq = nil
	}
SENDING:
	for {
		select {
		case <-s.closedq:
			break SENDING
		case <-p.stopq:
			break SENDING
		case msg = <-sendq:
		case msg = <-p.sendq:
		}

		if err = p.SendMsg(msg); err != nil {
			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithField("domain", "sender").
					WithError(err).
					WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw()}).
					Error("sendMsg")
			}
			if errx := s.resendMsg(msg); errx != nil {
				// free
				msg.FreeAll()
			}

			break SENDING
		}
		// free
		msg.FreeAll()
	}
	// seems can be moved to case <-s.closedq
	s.remPipe(p.ID())
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "sender").
			WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw()}).
			Debug("sender stopped run")
	}
}

func (s *sender) sendTo(msg *message.Message) (err error) {
	if msg.Header.Distance == 0 {
		// already arrived, just drop
		return
	}

	s.RLock()
	p := s.pipes[msg.Destination.CurID()]
	s.RUnlock()
	if p == nil {
		err = ErrBrokenPath
		return
	}

	return s.doPushMsg(msg, p.sendq)
}

func (s *sender) sendToAll(msg *message.Message) (err error) {
	s.RLock()
	for _, p := range s.pipes {
		s.doPushMsg(msg.Dup(), p.sendq)
	}
	s.RUnlock()
	msg.FreeAll()
	return nil
}

func (s *sender) Send(content []byte) (err error) {
	return s.doPushMsg(message.NewSendMessage(message.SendTypeToOne, nil, nil, 0, s.ttl(), content), s.sendq)
}

func (s *sender) SendTo(dest message.MsgPath, content []byte) (err error) {
	return s.sendTo(message.NewSendMessage(message.SendTypeToDest, nil, dest, 0, s.ttl(), content))
}

func (s *sender) SendAll(content []byte) (err error) {
	return s.sendToAll(message.NewSendMessage(message.SendTypeToAll, nil, nil, 0, s.ttl(), content))
}

func (s *sender) SendMsg(msg *message.Message) error {
	if msg.Header.TTL == 0 {
		// drop msg
		return nil
	}
	switch msg.Header.SendType() {
	case message.SendTypeToDest:
		return s.sendTo(msg)
	case message.SendTypeToOne:
		return s.doPushMsg(msg, s.sendq)
	case message.SendTypeToAll:
		return s.sendToAll(msg)
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
	connectors := make([]connector.Connector, 0, len(s.attachedConnectors))
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
