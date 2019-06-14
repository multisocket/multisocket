package stream

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/transport"

	log "github.com/sirupsen/logrus"

	"github.com/webee/multisocket"
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/receiver"
	"github.com/webee/multisocket/sender"
)

type (
	// RawToStream is a switch device which connect raw socket to stream.
	RawToStream struct {
		sync.Mutex
		stream    Stream
		rawSock   multisocket.Socket
		pipeConns map[uint32]*pipeConn
	}

	pipeConn struct {
		id      uint32
		dest    [4]byte
		s       multisocket.Socket
		conn    Connection
		closedq chan struct{}
		recvq   chan *multisocket.Message
	}
)

// NewRawToStream create a RawToStream switch device.
func NewRawToStream() *RawToStream {
	stream := NewWithOptions(options.NewOptions().WithOption(OptionAcceptable, false))
	rawSock := multisocket.New(connector.New(), sender.New(), receiver.New())

	rs := &RawToStream{
		stream:    stream,
		rawSock:   rawSock,
		pipeConns: make(map[uint32]*pipeConn),
	}

	go rs.run()
	return rs
}

// StreamDial dial for stream
func (rs *RawToStream) StreamDial(addr string) error {
	return rs.stream.Dial(addr)
}

// StreamListen listen for stream
func (rs *RawToStream) StreamListen(addr string) error {
	return rs.stream.Listen(addr)
}

// RawDial dial for raw sock
func (rs *RawToStream) RawDial(addr string) error {
	return rs.rawSock.DialOptions(addr, options.NewOptions().WithOption(transport.OptionConnRawMode, true))
}

// RawListen listen for raw sock
func (rs *RawToStream) RawListen(addr string) error {
	return rs.rawSock.ListenOptions(addr, options.NewOptions().WithOption(transport.OptionConnRawMode, true))
}

func (rs *RawToStream) run() {
	var (
		err error
		msg *multisocket.Message
	)

	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "RawToStream").
			WithField("action", "start").
			Debug("run")
	}

	for {
		if msg, err = rs.rawSock.RecvMsg(); err != nil {
			break
		}
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithField("domain", "RawToStream").
				WithFields(log.Fields{"content": msg.Content}).
				Debug("run")
		}
		pid, _ := msg.Source.CurID()
		rs.Lock()
		pc := rs.pipeConns[pid]
		rs.Unlock()
		if pc == nil {
			pc = rs.newPipeConn(pid)
			rs.Lock()
			rs.pipeConns[pid] = pc
			rs.Unlock()
			go rs.runPipeConn(pc)
		}
		select {
		case <-pc.closedq:
		case pc.recvq <- msg:
		}
	}
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "RawToStream").
			WithField("action", "end").
			Debug("run")
	}
}

// Close close
func (rs *RawToStream) Close() {
	rs.stream.Close()
	rs.rawSock.Close()
}

func (rs *RawToStream) newPipeConn(id uint32) *pipeConn {
	pc := &pipeConn{
		id:      id,
		dest:    [4]byte{},
		s:       rs.rawSock,
		closedq: make(chan struct{}),
		recvq:   make(chan *multisocket.Message, 64),
	}

	binary.BigEndian.PutUint32(pc.dest[:4], id)

	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "RawToStream").
			WithFields(log.Fields{"id": id}).
			Debug("newPipeConn")
	}
	return pc
}

func (rs *RawToStream) runPipeConn(pc *pipeConn) {
	var (
		err  error
		conn Connection
	)
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "RawToStream").
			WithFields(log.Fields{"id": pc.id, "action": "connecting"}).
			Debug("runPipeConn")
	}
	if conn, err = rs.stream.Connect(0); err != nil {
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithField("domain", "RawToStream").
				WithError(err).
				WithFields(log.Fields{"id": pc.id, "action": "connect failed"}).
				Debug("runPipeConn")
		}
		pc.close()
		return
	}
	pc.conn = conn
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "RawToStream").
			WithFields(log.Fields{"id": pc.id, "action": "connected"}).
			Debug("runPipeConn")
	}

	dest := multisocket.MsgPath(nil)
	dest = dest.AddSource(pc.dest)
	// stream to raw
	go func() {
		var (
			err error
			n   int
			p   = make([]byte, 4*1024)
		)
		msg := multisocket.NewMessage(multisocket.SendTypeToDest, dest, 0, nil)
		for {
			// if stream close, read will return error
			if n, err = pc.conn.Read(p); n > 0 {
				msg.Content = p[:n]
				err = pc.s.SendMsg(msg)
			}

			if err != nil {
				pc.close()
				break
			}
		}
	}()

	// raw to stream
	testMsg := multisocket.NewMessage(multisocket.SendTypeToDest, dest, 0, nil)
	tq := time.After(5 * time.Second)
RAW_TO_STREAM:
	for {
		select {
		case <-pc.closedq:
			break RAW_TO_STREAM
		case msg := <-pc.recvq:
			if _, err = pc.conn.Write(msg.Content); err != nil {
				pc.close()
				break RAW_TO_STREAM
			}
		case <-tq:
			// check if raw pipe disconnected
			if err := pc.s.SendMsg(testMsg); err != nil {
				pc.close()
				break RAW_TO_STREAM
			}
			tq = time.After(5 * time.Second)
		}
	}
}

func (pc *pipeConn) close() {
	select {
	case <-pc.closedq:
		return
	default:
		close(pc.closedq)
	}
	if pc.conn != nil {
		// close stream connection
		pc.conn.Close()
	}
	// close raw pipe
	pc.s.ClosePipe(pc.id)
}
