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
		closedq chan struct{}
		conns   map[uint32]*connection
		connq   chan *connection

		streamRecvQueueSize int
		acceptable          bool
	}

	connection struct {
		id      uint32
		closedq chan struct{}
		recvq   chan *message.Message
		s       *stream
		src     message.MsgPath
		dest    [4]byte
		pr      *io.PipeReader
		pw      *io.PipeWriter
	}
)

const (
	defaultConnQueueSize         = 16
	defaultConnRecvQueueSize     = 64
	defaultConnKeepAliveIdle     = 60 * time.Second
	defaultConnKeepAliveInterval = 2 * time.Second
	defaultConnKeepAliveProbes   = 9
)

var (
	nilTq        <-chan time.Time
	pipeStreamID = utils.NewRecyclableIDGenerator()
)

// New create a Stream protocol instance
func New() Stream {
	return NewWithOptions(options.NewOptions())
}

// NewWithOptions create a Stream protocol instance with options
func NewWithOptions(opts options.Options) Stream {
	streamQueueSize := OptionStreamQueueSize.Value(opts.GetOptionDefault(OptionStreamQueueSize, defaultConnQueueSize))
	streamRecvQueueSize := OptionConnRecvQueueSize.Value(opts.GetOptionDefault(OptionConnRecvQueueSize, defaultConnRecvQueueSize))
	acceptable := OptionAcceptable.Value(opts.GetOptionDefault(OptionAcceptable, true))
	s := &stream{
		Options:             opts,
		Socket:              multisocket.New(connector.New(), sender.New(), receiver.New()),
		closedq:             make(chan struct{}),
		conns:               make(map[uint32]*connection),
		connq:               make(chan *connection, streamQueueSize),
		streamRecvQueueSize: streamRecvQueueSize,
		acceptable:          acceptable,
	}

	go s.run()
	return s
}

// options
func (s *stream) connKeepAliveIdle() time.Duration {
	return OptionConnKeepAliveIdle.Value(s.GetOptionDefault(OptionConnKeepAliveIdle, defaultConnKeepAliveIdle))
}

func (s *stream) connKeepAliveInterval() time.Duration {
	return OptionConnKeepAliveInterval.Value(s.GetOptionDefault(OptionConnKeepAliveInterval, defaultConnKeepAliveInterval))
}

func (s *stream) connKeepAliveProbes() int {
	return OptionConnKeepAliveProbes.Value(s.GetOptionDefault(OptionConnKeepAliveProbes, defaultConnKeepAliveProbes))
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
				WithField("content", msg.Content).Info("recv")
		}
		switch msg.Header.SendType() {
		case message.SendTypeToDest:
			// find stream
			if id, ok = msg.Destination.CurID(); !ok {
				continue
			}

			conn = s.getConn(id)
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

			// new connection
			conn = s.newConnection(msg.Source)
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

		select {
		case <-s.closedq:
			break RUNNING
		case <-conn.closedq:
		case conn.recvq <- msg:
		}
	}
}

func (s *stream) getConn(id uint32) (c *connection) {
	s.RLock()
	c = s.conns[id]
	s.RUnlock()
	return
}

func (s *stream) addConn(c *connection) {
	s.Lock()
	s.conns[c.id] = c
	s.Unlock()

	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithField("streamID", c.id).Info("addConn")
	}
}

func (s *stream) remConn(id uint32) {
	s.Lock()
	delete(s.conns, id)
	s.Unlock()

	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithField("streamID", id).Info("remConn")
	}
}

func (s *stream) Connect(timeout time.Duration) (conn Connection, err error) {
	// new stream
	c := s.newConnection(nil)
	s.addConn(c)

	defer func() {
		if err != nil {
			s.remConn(c.id)
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

	tq := nilTq
	if timeout > 0 {
		tq = time.After(timeout)
	}
	select {
	case <-tq:
		err = errs.ErrTimeout
		return
	case msg := <-c.recvq:
		c.src = msg.Source
		c.recvq <- msg
		go c.run()
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithField("domain", "stream").
				WithField("streamID", c.id).
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
	select {
	case <-s.closedq:
		return errs.ErrClosed
	default:
		close(s.closedq)
		return s.Socket.Close()
	}
}

func (s *stream) newConnection(src message.MsgPath) *connection {
	pr, pw := io.Pipe()
	conn := &connection{
		id:      pipeStreamID.NextID(),
		closedq: make(chan struct{}),
		recvq:   make(chan *message.Message, s.streamRecvQueueSize),
		s:       s,
		src:     src,
		dest:    [4]byte{},
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
			WithField("streamID", conn.id).
			Info(">>connect")
	}

	msg := message.NewMessage(message.SendTypeToOne, nil, message.MsgFlagControl, []byte(ControlMsgKeepAlive))
	msg.AddSource(conn.dest)
	return conn.s.SendMsg(msg)
}

func (conn *connection) sendKeepAlive() error {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithField("streamID", conn.id).
			Info(">keepAlive")
	}

	msg := message.NewMessage(message.SendTypeToDest, conn.src, message.MsgFlagControl, []byte(ControlMsgKeepAlive))
	msg.AddSource(conn.dest)
	return conn.s.SendMsg(msg)
}

func (conn *connection) sendKeepAliveAck() error {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithField("streamID", conn.id).
			Info(">keepAliveAck")
	}

	msg := message.NewMessage(message.SendTypeToDest, conn.src, message.MsgFlagControl, []byte(ControlMsgKeepAliveAck))
	msg.AddSource(conn.dest)
	return conn.s.SendMsg(msg)
}

func (conn *connection) run() {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithField("streamID", conn.id).
			WithField("action", "start").Info("run")
	}

	var (
		msg            *message.Message
		keepAliveTq    <-chan time.Time
		keepAliveAckTq <-chan time.Time
	)
	keepAliveIdle := conn.s.connKeepAliveIdle()
	keepAliveInterval := conn.s.connKeepAliveInterval()
	keepAliveProbes := conn.s.connKeepAliveProbes()

	keepAliveTq = time.After(keepAliveIdle)
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
							WithField("streamID", conn.id).
							Info("<keepAlive")
					}
					if err := conn.sendKeepAliveAck(); err != nil {
						conn.Close()
						break RUNNING
					}
				case ControlMsgKeepAliveAck:
					if log.IsLevelEnabled(log.DebugLevel) {
						log.WithField("domain", "stream").
							WithField("streamID", conn.id).
							Info("<keepAliveAck")
					}
				default:
					if log.IsLevelEnabled(log.DebugLevel) {
						log.WithField("domain", "stream").
							WithField("streamID", conn.id).
							Info("<emptyControl")
					}
				}
			} else {
				conn.pw.Write(msg.Content)
			}
			// clear keepAliveAckTq, reset keepAliveTq
			probes = keepAliveProbes
			keepAliveAckTq = nil
			keepAliveTq = time.After(conn.s.connKeepAliveIdle())
		case <-keepAliveTq:
			probes--
			// time to send heartbeat
			if err := conn.sendKeepAlive(); err != nil {
				conn.Close()
				break RUNNING
			}

			// setup keepAliveAckTq, reset keepAliveTq
			keepAliveAckTq = nil
			if keepAliveInterval > 0 {
				keepAliveAckTq = time.After(keepAliveInterval)
			}
			keepAliveTq = time.After(conn.s.connKeepAliveIdle())
		case <-keepAliveAckTq:
			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithField("domain", "stream").
					WithField("streamID", conn.id).
					WithField("msg", "keepAlive").Info("timeout")
			}

			if probes > 0 {
				probes--
				// time to send heartbeat
				if err := conn.sendKeepAlive(); err != nil {
					conn.Close()
					break RUNNING
				}

				// setup keepAliveAckTq
				keepAliveAckTq = nil
				if keepAliveInterval > 0 {
					keepAliveAckTq = time.After(keepAliveInterval)
				}
			} else {
				// recv heartbeat timeout
				conn.Close()
				break RUNNING
			}
		}
	}

	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithField("streamID", conn.id).
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
	msg := message.NewMessage(message.SendTypeToDest, conn.src, 0, content)
	msg.AddSource(conn.dest)
	if err = conn.s.SendMsg(msg); err != nil {
		conn.Close()
		return
	}
	n = len(content)
	return
}

func (conn *connection) Close() error {
	select {
	case <-conn.closedq:
		return errs.ErrClosed
	default:
		close(conn.closedq)
	}

	s := conn.s
	conn.pw.Close()
	conn.pr.Close()

	s.remConn(conn.id)

	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithField("streamID", conn.id).Info("close")
	}

	return nil
}
