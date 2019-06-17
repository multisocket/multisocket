package reqrep

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/webee/multisocket/message"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket"
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/errs"
	"github.com/webee/multisocket/receiver"
	"github.com/webee/multisocket/sender"
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

func (goRunner) Run(f func()) {
	go f()
}

// NewRep create a Rep protocol instance
func NewRep(handler Handler) Rep {
	return &rep{
		Socket:  multisocket.New(connector.New(), sender.New(), receiver.New()),
		handler: handler,
		runner:  defaultRunner,
	}
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
	var (
		err error
		msg *message.Message
	)
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("action", "start").Debug("run")
	}
	for {
		if msg, err = r.RecvMsg(); err != nil {
			break
		}

		r.runner.Run(func() { r.handle(msg) })
	}
}

func (r *rep) handle(msg *message.Message) {
	requestID := msg.Content[:4]
	if log.IsLevelEnabled(log.TraceLevel) {
		log.WithFields(log.Fields{"requestID": binary.BigEndian.Uint32(requestID), "source": fmt.Sprintf("0x%x", msg.Source)}).
			WithField("action", "start").Trace("handle")
	}
	rep := r.handler.Handle(msg.Content[4:])
	if err := r.SendTo(msg.Source, requestID, rep); err != nil {
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithFields(log.Fields{"requestID": binary.BigEndian.Uint32(requestID), "source": fmt.Sprintf("0x%x", msg.Source)}).
				WithField("action", "send").Debug("handle")
		}
	}
	if log.IsLevelEnabled(log.TraceLevel) {
		log.WithFields(log.Fields{"requestID": binary.BigEndian.Uint32(requestID), "source": fmt.Sprintf("0x%x", msg.Source)}).
			WithField("action", "done").Trace("handle")
	}
}
