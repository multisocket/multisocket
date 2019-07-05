package reqrep

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/webee/multisocket/message"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket"
	"github.com/webee/multisocket/errs"
)

type (
	rep struct {
		multisocket.Socket
		handler Handler
		runner  Runner

		sync.Mutex
		started bool
		closed  bool
	}
	goRunner int
)

const (
	defaultRunner = goRunner(0)
)

var (
	bytesBufferPool = &sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
)

func (goRunner) Run(f func()) {
	go f()
}

// NewRep create a Rep protocol instance
func NewRep(handler Handler) Rep {
	return &rep{
		Socket:  multisocket.NewDefault(),
		handler: handler,
		runner:  defaultRunner,
	}
}

func (r *rep) GetSocket() multisocket.Socket {
	return r.Socket
}

func (r *rep) SetRunner(runner Runner) {
	r.Lock()
	defer r.Unlock()
	if r.started {
		return
	}
	if runner != nil {
		r.runner = runner
	}
}

func (r *rep) Start() {
	r.Lock()
	defer r.Unlock()
	if r.started {
		return
	}
	r.started = true
	go r.run()
}

func (r *rep) Close() error {
	r.Lock()
	defer r.Unlock()
	if r.closed {
		return errs.ErrClosed
	}
	r.closed = true
	return r.Socket.Close()
}

func (r *rep) run() {
	for {
		msg, err := r.RecvMsg()
		if err != nil {
			break
		}

		r.runner.Run(func() { r.handle(msg) })
	}
}

func (r *rep) handle(msg *message.Message) {
	buf := bytesBufferPool.Get().(*bytes.Buffer)
	// requestID
	buf.Write(msg.Content[:4])
	buf.Write(r.handler.Handle(msg.Content[4:]))
	if err := r.SendTo(msg.Source, buf.Bytes()); err != nil {
		if log.IsLevelEnabled(log.ErrorLevel) {
			requestID := msg.Content[:4]
			log.WithError(err).WithFields(log.Fields{"requestID": binary.BigEndian.Uint32(requestID), "source": fmt.Sprintf("0x%x", msg.Source)}).
				Error("reply")
		}
	}
	// free
	buf.Reset()
	bytesBufferPool.Put(buf)
	msg.FreeAll()
}
