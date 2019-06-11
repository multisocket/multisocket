package rep

import (
	"encoding/binary"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket"
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/receiver"
	"github.com/webee/multisocket/sender"
)

type (
	// Handler is request handler
	Handler interface {
		Handle(req []byte) (rep []byte)
	}
	// Runner is handler runner
	Runner interface {
		Run(func())
	}

	// Rep is the Rep protocol
	Rep interface {
		multisocket.ConnectorAction

		// actions
		SetRunner(runner Runner)
		Start()
		Close()
	}

	rep struct {
		multisocket.Socket
		handler Handler
		runner  Runner

		sync.Mutex
		started bool
		closed  bool
	}
)

// New create a Rep protocol instance
func New(handler Handler) Rep {
	return &rep{
		Socket:  multisocket.New(connector.New(), sender.New(), receiver.New()),
		handler: handler,
	}
}

func (r *rep) SetRunner(runner Runner) {
	r.Lock()
	defer r.Unlock()
	if r.started {
		return
	}
	r.runner = runner
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

func (r *rep) Close() {
	r.Lock()
	defer r.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	r.Socket.Close()
}

func (r *rep) run() {
	var (
		err error
		msg *multisocket.Message
	)
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("action", "start").Debug("run")
	}
	for {
		if msg, err = r.RecvMsg(); err != nil {
			break
		}
		if r.runner != nil {
			r.runner.Run(func() { r.handle(msg) })
		} else {
			go r.handle(msg)
		}
	}
}

func (r *rep) handle(msg *multisocket.Message) {
	requestID := msg.Content[:4]
	if log.IsLevelEnabled(log.TraceLevel) {
		log.WithField("requestID", binary.BigEndian.Uint32(requestID)).
			WithField("action", "start").Trace("handle")
	}
	rep := r.handler.Handle(msg.Content[4:])
	if err := r.SendTo(msg.Source, requestID, rep); err != nil {
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithError(err).WithField("requestID", binary.BigEndian.Uint32(requestID)).
				WithField("action", "send").Debug("handle")
		}
	}
	if log.IsLevelEnabled(log.TraceLevel) {
		log.WithField("requestID", binary.BigEndian.Uint32(requestID)).
			WithField("action", "done").Trace("handle")
	}
}
