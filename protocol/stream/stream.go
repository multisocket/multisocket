package stream

import (
	"encoding/binary"
	"io"
	"sync"
	"time"

	"github.com/webee/multisocket/message"

	"github.com/webee/multisocket/utils"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/receiver"
	"github.com/webee/multisocket/sender"

	"github.com/webee/multisocket"
	"github.com/webee/multisocket/errs"
)

type (
	stream struct {
		options.Options
		multisocket.Socket

		sync.RWMutex
		closedq  chan struct{}
		conns    map[uint32]*connection
		rawConns map[uint32]*connection
		connq    chan *connection

		streamRecvQueueSize int
		acceptable          bool
	}

	connection struct {
		sync.Mutex
		s       *stream
		id      uint32
		dest    [4]byte // id bytes
		closedq chan struct{}
		recvq   chan *message.Message
		src     message.MsgPath
		raw     bool
		pr      *io.PipeReader
		pw      *io.PipeWriter
	}
)

var (
	pipeStreamID = utils.NewRecyclableIDGenerator()
)

// New create a Stream protocol instance
func New() Stream {
	return NewWithOptions(nil)
}

// NewWithOptions create a Stream protocol instance with options
func NewWithOptions(ovs options.OptionValues) Stream {
	opts := options.NewOptionsWithValues(ovs)
	streamQueueSize := Options.StreamQueueSize.ValueFrom(opts)
	streamRecvQueueSize := Options.ConnRecvQueueSize.ValueFrom(opts)
	acceptable := Options.Acceptable.ValueFrom(opts)
	s := &stream{
		Options:             opts,
		Socket:              multisocket.New(connector.New(), sender.New(), receiver.New()),
		closedq:             make(chan struct{}),
		conns:               make(map[uint32]*connection),
		rawConns:            make(map[uint32]*connection),
		connq:               make(chan *connection, streamQueueSize),
		streamRecvQueueSize: streamRecvQueueSize,
		acceptable:          acceptable,
	}

	go s.run()
	return s
}

// options
func (s *stream) connKeepAliveIdle() time.Duration {
	return Options.ConnKeepAliveIdle.ValueFrom(s.Options)
}

func (s *stream) connKeepAliveInterval() time.Duration {
	return Options.ConnKeepAliveInterval.ValueFrom(s.Options)
}

func (s *stream) connKeepAliveProbes() int {
	return Options.ConnKeepAliveProbes.ValueFrom(s.Options)
}

func (s *stream) run() {
	var (
		id   uint32
		ok   bool
		err  error
		msg  *message.Message
		conn *connection
	)
RUNNING:
	for {
		if msg, err = s.RecvMsg(); err != nil {
			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithField("domain", "stream").
					WithError(err).Info("RecvMsg error")
			}
			break RUNNING
		}
		if log.IsLevelEnabled(log.TraceLevel) {
			log.WithField("domain", "stream").
				WithField("content", string(msg.Content)).Info("recv")
		}
		switch msg.Header.SendType() {
		case message.SendTypeToDest:
			// find stream
			conn = nil
			if id, ok = msg.Destination.CurID(); ok {
				conn = s.getConn(id, false)
			}

			if conn == nil {
				if log.IsLevelEnabled(log.DebugLevel) {
					log.WithField("domain", "stream").
						WithField("streamID", id).Warn("stream not found")
				}
				continue RUNNING
			}
		default:
			if !s.acceptable {
				continue RUNNING
			}

			conn = nil
			raw := msg.Header.HasFlags(message.MsgFlagRaw)
			if raw {
				// raw
				id = msg.PipeID()
				conn = s.getConn(id, raw)
			} else {
				id = pipeStreamID.NextID()
			}

			if conn == nil {
				// new connection
				conn = s.newConnection(id, msg.Source, raw)
				s.addConn(conn)

				if log.IsLevelEnabled(log.DebugLevel) {
					log.WithField("domain", "stream").
						WithField("streamID", conn.id).Info("new stream")
				}

				go conn.run()

				select {
				case <-s.closedq:
					break RUNNING
				case s.connq <- conn:
				}
			}
		}

		select {
		case <-s.closedq:
			break RUNNING
		case <-conn.closedq:
		case conn.recvq <- msg:
		}
	}
}

func (s *stream) getConn(id uint32, raw bool) (c *connection) {
	s.RLock()
	if raw {
		c = s.rawConns[id]
	} else {
		c = s.conns[id]
	}
	s.RUnlock()
	return
}

func (s *stream) addConn(c *connection) {
	s.Lock()
	if c.raw {
		s.rawConns[c.id] = c
	} else {
		s.conns[c.id] = c
	}
	s.Unlock()

	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithFields(log.Fields{"streamID": c.id, "raw": c.raw}).Info("addConn")
	}
}

func (s *stream) remConn(id uint32, raw bool) {
	s.Lock()
	if raw {
		delete(s.rawConns, id)
	} else {
		delete(s.conns, id)
	}
	s.Unlock()

	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithFields(log.Fields{"streamID": id, "raw": raw}).Info("remConn")
	}
}

func (s *stream) Connect(timeout time.Duration) (conn Connection, err error) {
	// new stream
	c := s.newConnection(pipeStreamID.NextID(), nil, false)
	s.addConn(c)

	defer func() {
		if err != nil {
			s.remConn(c.id, c.raw)
		}
	}()

	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithField("streamID", c.id).
			WithField("action", "connecting").
			Info("Connect")
	}

	if err = c.sendConnect(); err != nil {
		return
	}

	tm := utils.NewTimer()
	if timeout > 0 {
		tm.Reset(timeout)
	}
	select {
	case <-tm.C:
		err = errs.ErrTimeout
		return
	case msg := <-c.recvq:
		tm.Stop()

		c.src = msg.Source
		c.raw = msg.Header.HasFlags(message.MsgFlagRaw)
		c.recvq <- msg
		go c.run()
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithField("domain", "stream").
				WithFields(log.Fields{"streamID": c.id, "raw": c.raw}).
				WithField("action", "connected").
				Info("Connect")
		}
		return c, nil
	}
}

func (s *stream) Accept() (conn Connection, err error) {
	if !s.acceptable {
		err = errs.ErrOperationNotSupported
		return
	}

	select {
	case <-s.closedq:
		err = errs.ErrClosed
		return
	case conn = <-s.connq:
		return
	}
}

func (s *stream) Close() error {
	s.Lock()
	select {
	case <-s.closedq:
		s.Unlock()
		return errs.ErrClosed
	default:
		close(s.closedq)
	}
	s.Unlock()

	return s.Socket.Close()
}

func (s *stream) newConnection(id uint32, src message.MsgPath, raw bool) *connection {
	pr, pw := io.Pipe()
	conn := &connection{
		s:       s,
		id:      id,
		dest:    [4]byte{},
		closedq: make(chan struct{}),
		recvq:   make(chan *message.Message, s.streamRecvQueueSize),
		src:     src,
		raw:     raw,
		pr:      pr,
		pw:      pw,
	}
	binary.BigEndian.PutUint32(conn.dest[:4], conn.id)
	return conn
}

// used to start a stream connection
func (conn *connection) sendConnect() error {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
			Info(">>connect")
	}

	msg := message.NewMessage(message.SendTypeToOne, nil, message.MsgFlagControl, []byte(ControlMsgKeepAlive))
	msg.AddSource(conn.dest)
	return conn.s.SendMsg(msg)
}

func (conn *connection) sendKeepAlive() error {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
			Info(">keepAlive")
	}

	return conn.sendMsg([]byte(ControlMsgKeepAlive), message.MsgFlagControl)
}

func (conn *connection) sendKeepAliveAck() error {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
			Info(">keepAliveAck")
	}

	return conn.sendMsg([]byte(ControlMsgKeepAliveAck), message.MsgFlagControl)
}

func (conn *connection) run() {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
			WithField("action", "start").Info("run")
	}

	var (
		msg               *message.Message
		keepAliveTimer    = utils.NewTimer()
		keepAliveAckTimer = utils.NewTimer()
	)
	keepAliveIdle := conn.s.connKeepAliveIdle()
	keepAliveInterval := conn.s.connKeepAliveInterval()
	if conn.raw {
		keepAliveInterval = 0
	}
	keepAliveProbes := conn.s.connKeepAliveProbes()

	if keepAliveIdle > 0 {
		keepAliveTimer.Reset(keepAliveIdle)
	}
	probes := keepAliveProbes
RUNNING:
	for {
		select {
		case <-conn.closedq:
			break RUNNING
		case msg = <-conn.recvq:
			if msg.Header.HasFlags(message.MsgFlagControl) {
				// handle control msg
				switch string(msg.Content) {
				case ControlMsgKeepAlive:
					if log.IsLevelEnabled(log.DebugLevel) {
						log.WithField("domain", "stream").
							WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
							Info("<keepAlive")
					}
					if err := conn.sendKeepAliveAck(); err != nil {
						break RUNNING
					}
				case ControlMsgKeepAliveAck:
					if log.IsLevelEnabled(log.DebugLevel) {
						log.WithField("domain", "stream").
							WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
							Info("<keepAliveAck")
					}
				default:
					if log.IsLevelEnabled(log.DebugLevel) {
						log.WithField("domain", "stream").
							WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
							WithField("content", string(msg.Content)).
							Info("<unknownControl")
					}
				}
			} else {
				if len(msg.Content) > 0 {
					// FIXME: send before read will block here.
					conn.pw.Write(msg.Content)
				}
			}
			// clear keepAliveAckTimer, reset keepAliveTimer
			probes = keepAliveProbes
			keepAliveAckTimer.Stop()
			if keepAliveIdle > 0 {
				keepAliveTimer.Reset(keepAliveIdle)
			} else {
				keepAliveTimer.Stop()
			}
		case <-keepAliveTimer.C:
			probes--
			// time to send heartbeat
			if err := conn.sendKeepAlive(); err != nil {
				break RUNNING
			}

			// setup keepAliveAckTimer
			if keepAliveInterval > 0 {
				keepAliveAckTimer.Reset(keepAliveInterval)
			} else {
				keepAliveAckTimer.Stop()

				// no check ack, so reset keepAliveTimer
				if keepAliveIdle > 0 {
					keepAliveTimer.Reset(keepAliveIdle)
				} else {
					keepAliveTimer.Stop()
				}
			}
		case <-keepAliveAckTimer.C:
			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithField("domain", "stream").
					WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
					WithFields(log.Fields{"msg": "keepAliveAck", "probes": probes}).Info("timeout")
			}

			if probes > 0 {
				probes--
				// time to send heartbeat
				if err := conn.sendKeepAlive(); err != nil {
					break RUNNING
				}

				// setup keepAliveAckTq
				if keepAliveInterval > 0 {
					keepAliveAckTimer.Reset(keepAliveInterval)
				} else {
					keepAliveAckTimer.Stop()
				}
			} else {
				// recv heartbeat timeout
				break RUNNING
			}
		}
	}
	keepAliveTimer.Stop()
	keepAliveAckTimer.Stop()
	conn.Close()

	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
			WithField("action", "end").Info("run")
	}
}

func (conn *connection) Read(p []byte) (n int, err error) {
	return conn.pr.Read(p)
}

func (conn *connection) Write(p []byte) (n int, err error) {
	select {
	case <-conn.closedq:
		err = errs.ErrClosed
		return
	default:
	}

	if log.IsLevelEnabled(log.TraceLevel) {
		log.WithField("domain", "stream").
			WithField("content", p).Info("write")
	}

	// NOTE: Writer must not retain p
	content := append([]byte(nil), p...)
	if err = conn.sendMsg(content, 0); err != nil {
		conn.Close()
		return
	}
	n = len(content)
	return
}

func (conn *connection) sendMsg(content []byte, flags uint8) (err error) {
	msg := message.NewMessage(message.SendTypeToDest, conn.src, flags, content)
	msg.AddSource(conn.dest)
	err = conn.s.SendMsg(msg)
	if log.IsLevelEnabled(log.DebugLevel) && err != nil {
		log.WithField("domain", "stream").
			WithError(err).Error("sendMsg")
	}
	return
}

func (conn *connection) Close() error {
	conn.Lock()
	select {
	case <-conn.closedq:
		conn.Unlock()
		return errs.ErrClosed
	default:
		close(conn.closedq)
	}
	conn.Unlock()

	conn.pr.Close()
	conn.pw.Close()

	s := conn.s
	s.remConn(conn.id, conn.raw)
	// TODO: use internal msg to close peer, current we use hearbeat check.
	if conn.raw {
		s.Socket.ClosePipe(conn.id)
	}

	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
			Info("close")
	}

	return nil
}

func (conn *connection) Closed() bool {
	select {
	case <-conn.closedq:
		return true
	default:
		return false
	}
}
