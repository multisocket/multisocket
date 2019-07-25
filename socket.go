package multisocket

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
	socket struct {
		options.Options
		connector connector.Connector
		ConnectorAction

		sync.RWMutex
		closedq chan struct{}

		pipes map[uint32]*pipe

		// recv
		noRecv bool
		recvq  chan *message.Message
		// send
		noSend         bool
		ttl            uint8
		bestEffort     bool
		sendq          chan *message.Message
		senderWg       *sync.WaitGroup
		senderStopTm   *utils.Timer
		senderStoppedq chan struct{}
	}

	pipe struct {
		connector.Pipe
		// send
		stopq     chan struct{}
		sendq     chan *message.Message
		freeLevel message.FreeLevel
	}
)

var (
	emptyByteSlice = make([]byte, 0)
)

// NewNoSend create a no send Socket
func NewNoSend(ovs options.OptionValues) Socket {
	ovs[Options.NoSend] = true
	return New(ovs)
}

// NewNoRecv create a no recv Socket
func NewNoRecv(ovs options.OptionValues) Socket {
	ovs[Options.NoRecv] = true
	return New(ovs)
}

// NewDefault creates a default Socket
func NewDefault() Socket {
	return New(nil)
}

// New creates a Socket
func New(ovs options.OptionValues) Socket {
	s := &socket{
		Options: options.NewOptionsWithValues(ovs),
		closedq: make(chan struct{}),
		pipes:   make(map[uint32]*pipe),
		// send
		senderWg:       &sync.WaitGroup{},
		senderStopTm:   utils.NewTimer(),
		senderStoppedq: make(chan struct{}),
	}
	s.connector = connector.NewWithOptions(s.Options)
	s.ConnectorAction = s.connector
	// init option values
	s.onOptionChange(Options.NoRecv, nil, nil)
	s.onOptionChange(Options.RecvQueueSize, nil, nil)
	s.onOptionChange(Options.NoSend, nil, nil)
	s.onOptionChange(Options.SendQueueSize, nil, nil)
	s.onOptionChange(Options.SendTTL, nil, nil)
	s.onOptionChange(Options.SendBestEffort, nil, nil)

	s.Options.AddOptionChangeHook(s.onOptionChange)

	// set pipe event handler
	s.connector.SetPipeEventHandler(s.HandlePipeEvent)

	return s
}

// options

func (s *socket) onOptionChange(opt options.Option, oldVal, newVal interface{}) error {
	switch opt {
	case Options.NoRecv:
		s.noRecv = s.GetOptionDefault(Options.NoRecv).(bool)
	case Options.RecvQueueSize:
		s.recvq = make(chan *message.Message, s.recvQueueSize())
	case Options.NoRecv:
		s.noSend = s.GetOptionDefault(Options.NoSend).(bool)
	case Options.SendQueueSize:
		s.sendq = make(chan *message.Message, s.sendQueueSize())
	case Options.SendTTL:
		s.ttl = s.GetOptionDefault(Options.SendTTL).(uint8)
	case Options.SendBestEffort:
		s.bestEffort = s.GetOptionDefault(Options.SendBestEffort).(bool)
	}
	return nil
}

func (s *socket) recvQueueSize() uint16 {
	return s.GetOptionDefault(Options.RecvQueueSize).(uint16)
}

func (s *socket) sendQueueSize() uint16 {
	return s.GetOptionDefault(Options.SendQueueSize).(uint16)
}

func (s *socket) sendStopTimeout() time.Duration {
	return s.GetOptionDefault(Options.SendStopTimeout).(time.Duration)
}

func (s *socket) HandlePipeEvent(e connector.PipeEvent, pipe connector.Pipe) {
	switch e {
	case connector.PipeEventAdd:
		s.addPipe(pipe)
	case connector.PipeEventRemove:
		s.remPipe(pipe.ID())
	}
}

func (s *socket) addPipe(cp connector.Pipe) {
	s.Lock()
	p := s.newPipe(cp)
	s.pipes[p.ID()] = p
	go s.receiver(p)
	go s.sender(p)
	s.Unlock()
}

func (s *socket) newPipe(cp connector.Pipe) *pipe {
	return &pipe{
		Pipe: cp,
		// send
		stopq:     make(chan struct{}),
		sendq:     make(chan *message.Message, s.sendQueueSize()),
		freeLevel: cp.MsgFreeLevel(),
	}
}

func (s *socket) remPipe(id uint32) {
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

func (s *socket) stopPipe(p *pipe) {
	close(p.stopq)
	tm := utils.NewTimerWithDuration(s.sendStopTimeout())
	defer tm.Stop()
DRAIN_MSG_LOOP:
	for {
		select {
		case <-s.senderStoppedq:
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

// recv

func (s *socket) RecvMsg() (msg *message.Message, err error) {
	select {
	case <-s.closedq:
		// exhaust received messages
		select {
		case msg = <-s.recvq:
		default:
			err = errs.ErrClosed
		}
	case msg = <-s.recvq:
	}
	return
}

func (s *socket) receiver(p *pipe) {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "receiver").
			WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw()}).
			Debug("receiver start run")
	}

	var (
		err error
		msg *message.Message
	)

	if p.IsRaw() {
		// NOTE:
		// send a empty message to make a connection
		s.recvq <- message.NewRawRecvMessage(p.ID(), emptyByteSlice)
	}
RECVING:
	for {
		if msg, err = p.RecvMsg(); msg != nil {
			if s.noRecv {
				// just drop
				msg.FreeAll()
			} else if msg.HasFlags(message.MsgFlagInternal) {
				// FIXME: handle internal messages.
				msg.FreeAll()
			} else {
				select {
				case <-s.closedq:
					msg.FreeAll()
					s.remPipe(p.ID())
					break RECVING
				case s.recvq <- msg:
				}
			}
		}
		if err != nil {
			break RECVING
		}
	}

	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "receiver").
			WithError(err).
			WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw()}).
			Debug("receiver stopped run")
	}
}

// sender

func (s *socket) sender(p *pipe) {
	// start
	s.senderWg.Add(1)

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
				case <-s.senderStoppedq:
					// timeout
					break SEND_REMAINING
				default:
					break SEND_REMAINING
				}
			}
			s.remPipe(p.ID())
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
	// done
	s.senderWg.Done()
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "sender").
			WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw()}).
			Debug("sender stopped run")
	}
}

func (s *socket) doSendMsg(p *pipe, msg *message.Message) (err error) {
	if err = p.SendMsg(msg); err != nil {
		if s.resendMsg(msg) == nil {
			return
		}
		msg.FreeAll()
		return
	}
	msg.FreeByLevel(p.freeLevel)
	return
}

func (s *socket) doPushMsg(msg *message.Message, sendq chan<- *message.Message) (err error) {
	if s.bestEffort {
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

func (s *socket) resendMsg(msg *message.Message) error {
	if msg.SendType() == message.SendTypeToOne {
		// only resend when send to one, so we can choose another pipe to send.
		return s.doPushMsg(msg, s.sendq)
	}
	return errs.ErrBadMsg
}

func (s *socket) sendTo(msg *message.Message) (err error) {
	if msg.Distance == 0 {
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

func (s *socket) sendToAll(msg *message.Message) (err error) {
	s.RLock()
	for _, p := range s.pipes {
		s.doPushMsg(msg.Dup(), p.sendq)
	}
	s.RUnlock()
	msg.FreeAll()
	return nil
}

func (s *socket) Send(content []byte) (err error) {
	if s.noSend {
		return nil
	}
	return s.doPushMsg(message.NewSendMessage(0, message.SendTypeToOne, s.ttl, nil, nil, content), s.sendq)
}

func (s *socket) SendTo(dest message.MsgPath, content []byte) (err error) {
	if s.noSend {
		return nil
	}
	return s.sendTo(message.NewSendMessage(0, message.SendTypeToDest, s.ttl, nil, dest, content))
}

func (s *socket) SendAll(content []byte) (err error) {
	if s.noSend {
		return nil
	}

	return s.sendToAll(message.NewSendMessage(0, message.SendTypeToAll, s.ttl, nil, nil, content))
}

func (s *socket) SendMsg(msg *message.Message) error {
	if s.noSend {
		// drop msg
		msg.FreeAll()
		return nil
	}

	if msg.TTL == 0 {
		// drop msg
		msg.FreeAll()
		return nil
	}
	switch msg.SendType() {
	case message.SendTypeToDest:
		return s.sendTo(msg)
	case message.SendTypeToOne:
		return s.doPushMsg(msg, s.sendq)
	case message.SendTypeToAll:
		return s.sendToAll(msg)
	}
	return ErrInvalidSendType
}

func (s *socket) stopSender() {
	s.senderStopTm.Reset(s.sendStopTimeout())
	defer s.senderStopTm.Stop()

	// wait all pipes to stop
	done := make(chan struct{})
	go func() {
		s.senderWg.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-s.senderStopTm.C:
		close(s.senderStoppedq)
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

// connector

func (s *socket) Connector() connector.Connector {
	return s.connector
}

func (s *socket) Close() error {
	s.Lock()
	select {
	case <-s.closedq:
		s.Unlock()
		return errs.ErrClosed
	default:
		close(s.closedq)
	}
	s.Unlock()

	// clear pipe even handler
	s.connector.ClearPipeEventHandler(s.HandlePipeEvent)

	s.stopSender()
	s.connector.Close()

	return nil
}
