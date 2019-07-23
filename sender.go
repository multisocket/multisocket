package multisocket

import (
	"sync"
	"time"

	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/message"
	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/utils"
	log "github.com/sirupsen/logrus"
)

type (
	sender struct {
		socket *socket

		noSend     bool
		ttl        uint8
		bestEffort bool
		sendq      chan *message.Message
		wg         *sync.WaitGroup
		closetm    *utils.Timer
	}

	pipeSender struct {
		stopq chan struct{}
		sendq chan *message.Message
		// TODO: use a better way to support inproc.channel like transport: free payload internal
		freeAll bool
	}
)

func newSender(socket *socket) *sender {
	s := &sender{
		socket:  socket,
		wg:      &sync.WaitGroup{},
		closetm: utils.NewTimer(),
	}
	// init option values
	s.onOptionChange(Options.NoSend, nil, nil)
	s.onOptionChange(Options.SendQueueSize, nil, nil)
	s.onOptionChange(Options.SendTTL, nil, nil)
	s.onOptionChange(Options.SendBestEffort, nil, nil)

	return s
}

// options
func (s *sender) onOptionChange(opt options.Option, oldVal, newVal interface{}) error {
	switch opt {
	case Options.NoRecv:
		s.noSend = s.socket.GetOptionDefault(Options.NoSend).(bool)
	case Options.SendQueueSize:
		s.sendq = make(chan *message.Message, s.sendQueueSize())
	case Options.SendTTL:
		s.ttl = s.socket.GetOptionDefault(Options.SendTTL).(uint8)
	case Options.SendBestEffort:
		s.bestEffort = s.socket.GetOptionDefault(Options.SendBestEffort).(bool)
	}
	return nil
}

func (s *sender) sendQueueSize() uint16 {
	return s.socket.GetOptionDefault(Options.SendQueueSize).(uint16)
}

func (s *sender) closeTimeout() time.Duration {
	return s.socket.GetOptionDefault(Options.SendCloseTimeout).(time.Duration)
}

func (s *sender) stopPipe(p *pipe) {
	close(p.stopq)
	tm := utils.NewTimerWithDuration(s.closeTimeout())
	defer tm.Stop()
DRAIN_MSG_LOOP:
	for {
		select {
		case <-s.closetm.C:
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

func (s *sender) doPushMsg(msg *message.Message, sendq chan<- *message.Message) (err error) {
	if s.bestEffort {
		select {
		case <-s.socket.closedq:
			return errs.ErrClosed
		case sendq <- msg:
			return nil
		default:
			// drop msg
			return ErrMsgDropped
		}
	}

	select {
	case <-s.socket.closedq:
		err = errs.ErrClosed
	case sendq <- msg:
	}
	return
}

func (s *sender) resendMsg(msg *message.Message) error {
	if msg.SendType() == message.SendTypeToOne {
		// only resend when send to one, so we can choose another pipe to send.
		return s.doPushMsg(msg, s.sendq)
	}
	return errs.ErrBadMsg
}

func (s *sender) initPipe(p *pipe) {
	p.pipeSender = &pipeSender{
		stopq:   make(chan struct{}),
		sendq:   make(chan *message.Message, s.sendQueueSize()),
		freeAll: p.Transport().Scheme() != "inproc.channel",
	}
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
		case <-s.socket.closedq:
			// send remaining messages
		SEND_REMAINING:
			for {
				select {
				case msg = <-sendq:
					if err = s.doSendMsg(p, msg); err != nil {
						break SEND_REMAINING
					}
				case <-s.closetm.C:
					// timeout
					break SEND_REMAINING
				default:
					break SEND_REMAINING
				}
			}
			s.socket.remPipe(p.ID())
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
	s.wg.Done()
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "sender").
			WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw()}).
			Debug("sender stopped run")
	}
}

func (s *sender) doSendMsg(p *pipe, msg *message.Message) (err error) {
	if err = p.SendMsg(msg); err != nil {
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
	if msg.Distance == 0 {
		// already arrived, just drop
		return
	}

	p := s.socket.getPipeByID(msg.Destination.CurID())
	if p == nil {
		err = ErrBrokenPath
		return
	}

	return s.doPushMsg(msg, p.sendq)
}

func (s *sender) sendToAll(msg *message.Message) (err error) {
	s.socket.RLock()
	for _, p := range s.socket.pipes {
		s.doPushMsg(msg.Dup(), p.sendq)
	}
	s.socket.RUnlock()
	msg.FreeAll()
	return nil
}

func (s *sender) Send(content []byte) (err error) {
	if s.noSend {
		return nil
	}
	return s.doPushMsg(message.NewSendMessage(0, message.SendTypeToOne, s.ttl, nil, nil, content), s.sendq)
}

func (s *sender) SendTo(dest message.MsgPath, content []byte) (err error) {
	if s.noSend {
		return nil
	}
	return s.sendTo(message.NewSendMessage(0, message.SendTypeToDest, s.ttl, nil, dest, content))
}

func (s *sender) SendAll(content []byte) (err error) {
	if s.noSend {
		return nil
	}

	return s.sendToAll(message.NewSendMessage(0, message.SendTypeToAll, s.ttl, nil, nil, content))
}

func (s *sender) SendMsg(msg *message.Message) error {
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

func (s *sender) close() {
	s.closetm.Reset(s.closeTimeout())
	defer s.closetm.Stop()

	// wait all pipes to stop
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-s.closetm.C:
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
