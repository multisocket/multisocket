package sender

import (
	"net"
	"sync"

	"github.com/webee/multisocket/bytespool"

	"github.com/webee/multisocket/errs"

	"github.com/webee/multisocket/message"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/options"
)

type (
	sender struct {
		options.Options
		sendq chan *Message

		sync.RWMutex
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

var (
	netBufsPool = &sync.Pool{
		New: func() interface{} {
			return make(net.Buffers, 0, 4)
		},
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
	return s.GetOptionDefault(Options.TTL).(uint8)
}

func (s *sender) sendQueueSize() uint16 {
	return s.GetOptionDefault(Options.SendQueueSize).(uint16)
}

func (s *sender) bestEffort() bool {
	return s.GetOptionDefault(Options.SendBestEffort).(bool)
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
	p := s.newPipe(pipe)
	s.pipes[p.p.ID()] = p
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

func (s *sender) resendMsg(msg *Message) error {
	if msg.Header.SendType() == SendTypeToOne {
		// only resend when send to one, so we can choose another pipe to send.
		return s.SendMsg(msg)
	}
	return nil
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
			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithField("domain", "sender").
					WithError(err).
					WithFields(log.Fields{"id": p.p.ID(), "raw": p.p.IsRaw()}).
					Error("sendMsg")
			}
			if errx := s.resendMsg(msg); errx != nil {
				// free
				msg.FreeAll()
			}

			break SENDING
		} else {
			// free
			msg.FreeAll()
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

func (p *pipe) sendMsg(msg *Message) (err error) {
	if msg.Header.HasFlags(message.MsgFlagRaw) {
		// ignore raw messages. raw message is only for stream, forward raw message makes no sense.
		return nil
	}

	bufs := netBufsPool.Get().(net.Buffers)
	hbuf := msg.Header.Encode()
	bufs = append(bufs, hbuf, msg.Source, msg.Destination, msg.Content)
	_, err = bufs.WriteTo(p.p)

	// free
	// hbuf
	bytespool.Free(hbuf)

	// bufs
	bufs = bufs[:0]
	netBufsPool.Put(bufs)
	return
}

func (p *pipe) sendRawMsg(msg *Message) (err error) {
	if msg.Header.HasAnyFlags() {
		// ignore none normal messages.
		return
	}
	_, err = p.p.Write(msg.Content)

	return
}

func (s *sender) newMsg(sendType uint8, dest MsgPath, content []byte) (msg *Message) {
	return message.NewMessage(sendType, dest, 0, s.ttl(), content)
}

func (s *sender) sendTo(msg *Message) (err error) {
	var (
		id   uint32
		dest MsgPath
		ok   bool
		p    *pipe
	)
	if msg.Header.Distance == 0 {
		// already arrived, just drop
		return
	}

	if id, dest, ok = msg.Destination.NextID(); !ok {
		err = ErrBadDestination
		return
	}
	msg.Destination = dest
	msg.Header.Distance--

	s.Lock()
	p = s.pipes[id]
	s.Unlock()
	if p == nil {
		err = ErrBrokenPath
		return
	}

	return s.doPushMsg(msg, p.sendq)
}

func (s *sender) sendToAll(msg *Message) (err error) {
	s.RLock()
	for _, p := range s.pipes {
		s.doPushMsg(msg.Dup(), p.sendq)
	}
	s.RUnlock()
	msg.FreeAll()
	return nil
}

func (s *sender) Send(content []byte) (err error) {
	return s.doPushMsg(s.newMsg(SendTypeToOne, nil, content), s.sendq)
}

func (s *sender) SendTo(dest MsgPath, content []byte) (err error) {
	return s.sendTo(s.newMsg(SendTypeToDest, dest, content))
}

func (s *sender) SendAll(content []byte) (err error) {
	return s.sendToAll(s.newMsg(SendTypeToAll, nil, content))
}

func (s *sender) SendMsg(msg *Message) error {
	switch msg.Header.SendType() {
	case SendTypeToDest:
		return s.sendTo(msg)
	case SendTypeToOne:
		return s.doPushMsg(msg, s.sendq)
	case SendTypeToAll:
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
