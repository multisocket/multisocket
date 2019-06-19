package stream

import (
	"bytes"
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
		rbuf    *bytes.Buffer
		rq      chan struct{}
		wq      chan struct{}
	}
)

const (
	defaultConnQueueSize         = 16
	defaultConnRecvQueueSize     = 64
	defaultConnKeepAliveIdle     = 30 * time.Second
	defaultConnKeepAliveInterval = 1 * time.Second
	defaultConnKeepAliveProbes   = 7

	// TODO: optionize this values
	lowBufSize  = 4 * 1024
	highBufSize = 256 * 1024
	maxBufSize  = 1024 * 1024
)

var (
	nilTq        <-chan time.Time
	pipeStreamID = utils.NewRecyclableIDGenerator()
)

// New create a Stream protocol instance
func New() Stream {
	return NewWithOptions(nil)
}

// NewWithOptions create a Stream protocol instance with options
func NewWithOptions(ovs options.OptionValues) Stream {
	opts := options.NewOptionsWithValues(ovs)
	streamQueueSize := OptionStreamQueueSize.Value(opts.GetOptionDefault(OptionStreamQueueSize, defaultConnQueueSize))
	streamRecvQueueSize := OptionConnRecvQueueSize.Value(opts.GetOptionDefault(OptionConnRecvQueueSize, defaultConnRecvQueueSize))
	acceptable := OptionAcceptable.Value(opts.GetOptionDefault(OptionAcceptable, true))
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
				id, _ = msg.Source.CurID()
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
	conn := &connection{
		s:       s,
		id:      id,
		dest:    [4]byte{},
		closedq: make(chan struct{}),
		recvq:   make(chan *message.Message, s.streamRecvQueueSize),
		src:     src,
		raw:     raw,
		rbuf:    new(bytes.Buffer),
		rq:      make(chan struct{}),
		wq:      make(chan struct{}, 1),
	}
	// for write
	conn.wq <- struct{}{}
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

	msg := message.NewMessage(message.SendTypeToDest, conn.src, message.MsgFlagControl, []byte(ControlMsgKeepAlive))
	msg.AddSource(conn.dest)
	return conn.s.SendMsg(msg)
}

func (conn *connection) sendKeepAliveAck() error {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
			Info(">keepAliveAck")
	}

	msg := message.NewMessage(message.SendTypeToDest, conn.src, message.MsgFlagControl, []byte(ControlMsgKeepAliveAck))
	msg.AddSource(conn.dest)
	return conn.s.SendMsg(msg)
}

func (conn *connection) sendStopWriting() error {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
			Info(">stopWriting")
	}

	msg := message.NewMessage(message.SendTypeToDest, conn.src, message.MsgFlagControl, []byte(ControlMsgStopWriting))
	msg.AddSource(conn.dest)
	return conn.s.SendMsg(msg)
}

func (conn *connection) sendStartWriting() error {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
			Info(">startWriting")
	}

	msg := message.NewMessage(message.SendTypeToDest, conn.src, message.MsgFlagControl, []byte(ControlMsgStartWriting))
	msg.AddSource(conn.dest)
	return conn.s.SendMsg(msg)
}

func (conn *connection) run() {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
			WithField("action", "start").Info("run")
	}

	var (
		msg            *message.Message
		keepAliveTq    <-chan time.Time
		keepAliveAckTq <-chan time.Time
	)
	keepAliveIdle := conn.s.connKeepAliveIdle()
	keepAliveInterval := conn.s.connKeepAliveInterval()
	if conn.raw {
		keepAliveInterval = 0
	}
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
				case ControlMsgStopWriting:
					if log.IsLevelEnabled(log.DebugLevel) {
						log.WithField("domain", "stream").
							WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
							Info("<stopWriting")
					}
					select {
					case <-conn.wq:
					default:
					}
				case ControlMsgStartWriting:
					if log.IsLevelEnabled(log.DebugLevel) {
						log.WithField("domain", "stream").
							WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
							Info("<startWriting")
					}
					select {
					case conn.wq <- struct{}{}:
					default:
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
				// log.Infof("reading: [%s]", msg.Content)
				conn.reading(msg.Content)
			}
			// clear keepAliveAckTq, reset keepAliveTq
			probes = keepAliveProbes
			keepAliveAckTq = nil
			keepAliveTq = nil
			if keepAliveIdle > 0 {
				keepAliveTq = time.After(keepAliveIdle)
			}
		case <-keepAliveTq:
			probes--
			// time to send heartbeat
			if err := conn.sendKeepAlive(); err != nil {
				break RUNNING
			}

			// setup keepAliveAckTq, reset keepAliveTq
			keepAliveAckTq = nil
			if keepAliveInterval > 0 {
				keepAliveAckTq = time.After(keepAliveInterval)
			}
			keepAliveTq = nil
			if keepAliveIdle > 0 {
				keepAliveTq = time.After(keepAliveIdle)
			}
		case <-keepAliveAckTq:
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
				keepAliveAckTq = nil
				if keepAliveInterval > 0 {
					keepAliveAckTq = time.After(keepAliveInterval)
				}
			} else {
				// recv heartbeat timeout
				break RUNNING
			}
		}
	}
	conn.Close()

	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "stream").
			WithFields(log.Fields{"streamID": conn.id, "raw": conn.raw}).
			WithField("action", "end").Info("run")
	}
}

func (conn *connection) reading(p []byte) {
	sz := conn.rbuf.Len()
	conn.rbuf.Write(p)
	select {
	case conn.rq <- struct{}{}:
	default:
	}
	sz2 := conn.rbuf.Len()
	if sz2 >= maxBufSize {
		conn.Close()
	} else if sz < highBufSize && sz2 >= highBufSize {
		// notify stop writing
		if err := conn.sendStopWriting(); err != nil {
			conn.Close()
		}
	}
}

func (conn *connection) Read(p []byte) (n int, err error) {
	for {
		sz := conn.rbuf.Len()
		n, err = conn.rbuf.Read(p)
		if err == io.EOF {
			select {
			case <-conn.closedq:
				err = io.EOF
				return
			case <-conn.rq:
				continue
			}
		}

		if sz > lowBufSize && conn.rbuf.Len() <= lowBufSize {
			// notify start writing
			if err := conn.sendStartWriting(); err != nil {
				conn.Close()
			}
		}
		return
	}
}

func (conn *connection) Write(p []byte) (n int, err error) {
	select {
	case <-conn.closedq:
		err = errs.ErrClosed
		return
	case x := <-conn.wq:
		conn.wq <- x
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
	conn.Lock()
	select {
	case <-conn.closedq:
		conn.Unlock()
		return errs.ErrClosed
	default:
		close(conn.closedq)
	}
	conn.Unlock()

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
