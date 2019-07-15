package sender

import (
	"sync"
	"time"

	"github.com/multisocket/multisocket/connector"
	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/message"

	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/utils"
	log "github.com/sirupsen/logrus"
)

type (
	sender struct {
		options.Options
		sendq chan *message.Message

		sync.RWMutex
		closedq            chan struct{}
		attachedConnectors map[connector.Connector]struct{}
		pipes              map[uint32]*pipe
		wg                 *sync.WaitGroup
		closetq            <-chan time.Time
	}

	pipe struct {
		connector.Pipe
		stopq chan struct{}
		sendq chan *message.Message
		// TODO: use a better way to support inproc.channel like transport: free payload internal
		freeAll bool
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
		wg:                 &sync.WaitGroup{},
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
		Pipe:    cp,
		stopq:   make(chan struct{}),
		sendq:   make(chan *message.Message, s.sendQueueSize()),
		freeAll: cp.Transport().Scheme() != "inproc.channel",
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

func (s *sender) closeTimeout() time.Duration {
	return s.GetOptionDefault(Options.CloseTimeout).(time.Duration)
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

	s.stopPipe(p)
}

func (s *sender) stopPipe(p *pipe) {
	close(p.stopq)
	tm := utils.NewTimerWithDuration(s.closeTimeout())
	defer tm.Stop()
DRAIN_MSG_LOOP:
	for {
		select {
		case <-s.closetq:
			break DRAIN_MSG_LOOP
		case <-tm.C:
			break DRAIN_MSG_LOOP
		case msg := <-p.sendq:
			// send to dest/all msgs
			if err := s.doSendMsg(p, msg); err != nil {
				break DRAIN_MSG_LOOP
			}
		default:
			return
		}
	}
	// drop last
	for {
		select {
		case msg := <-p.sendq:
			msg.FreeAll()
		default:
			return
		}
	}
}

func (s *sender) resendMsg(msg *message.Message) error {
	if msg.Header.SendType() == message.SendTypeToOne {
		// only resend when send to one, so we can choose another pipe to send.
		return s.doPushMsg(msg, s.sendq)
	}
	return errs.ErrBadMsg
}

func (s *sender) run(p *pipe) {
	// start
	s.wg.Add(1)

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
			// send remaining messages
		SEND_REMAINING:
			for {
				select {
				case msg = <-sendq:
					if err = s.doSendMsg(p, msg); err != nil {
						break SEND_REMAINING
					}
				case <-s.closetq:
					// timeout
					break SEND_REMAINING
				default:
					break SEND_REMAINING
				}
			}
			break SENDING
		case <-p.stopq:
			break SENDING
		case msg = <-sendq:
		case msg = <-p.sendq:
		}

		if err = s.doSendMsg(p, msg); err != nil {
			break SENDING
		}
	}
	// seems can be moved to case <-s.closedq
	s.remPipe(p.ID())
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "sender").
			WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw()}).
			Debug("sender stopped run")
	}
	// done
	s.wg.Done()
}

func (s *sender) doSendMsg(p *pipe, msg *message.Message) (err error) {
	if err = p.SendMsg(msg); err != nil {
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithField("domain", "sender").
				WithError(err).
				WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw()}).
				Error("sendMsg")
		}

		if s.resendMsg(msg) == nil {
			return
		}
		msg.FreeAll()
		return
	}
	if p.freeAll {
		// free all
		msg.FreeAll()
	} else {
		msg.Free()
	}
	return
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
		s.closetq = time.After(s.closeTimeout())
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

	// wait all pipes to stop
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-s.closetq:
	}
	for {
		// drop remaining messages
		select {
		case msg := <-s.sendq:
			msg.FreeAll()
		default:
			return
		}
	}
}
