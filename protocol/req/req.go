package req

import (
	"encoding/binary"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket"
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/receiver"
	"github.com/webee/multisocket/sender"
)

type (
	// Request is a request object
	Request struct {
		r     *req
		ID    uint32
		Err   error
		Reply []byte
		Done  chan *Request
	}

	// Req is the Req protocol
	Req interface {
		multisocket.ConnectorAction

		// actions
		Request(content []byte) ([]byte, error)
		ReqeustTimeout(t time.Duration, content []byte) ([]byte, error)
		ReqeustAsync(content []byte) *Request
		Close()
	}

	req struct {
		multisocket.Socket
		timeout time.Duration

		sync.Mutex
		started  bool
		closed   bool
		reqID    uint32
		requests map[uint32]*Request
	}
)

const (
	defaultTimeout = time.Second * 5
)

// New create a Req protocol instance
func New() Req {
	return NewWithTimeout(defaultTimeout)
}

// NewWithTimeout create a Req protocol instance with request timeout
func NewWithTimeout(timeout time.Duration) Req {
	req := &req{
		Socket:   multisocket.New(connector.New(), sender.New(), receiver.New()),
		timeout:  timeout,
		reqID:    uint32(time.Now().UnixNano()), // quasi-random
		requests: make(map[uint32]*Request),
	}

	go req.run()
	return req
}

func (r *req) run() {
	var (
		err     error
		ok      bool
		msg     *multisocket.Message
		request *Request
	)
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("action", "start").Debug("run")
	}
	for {
		if msg, err = r.RecvMsg(); err != nil {
			break
		}
		id := binary.BigEndian.Uint32(msg.Content)
		if log.IsLevelEnabled(log.TraceLevel) {
			log.WithField("requestID", id).WithField("action", "start").Trace("recv")
		}
		r.Lock()
		if request, ok = r.requests[id]; !ok {
			r.Unlock()
			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithField("requestID", id).WithField("action", "miss").Debug("recv")
			}
			continue
		}
		delete(r.requests, id)
		r.Unlock()

		request.Reply = msg.Content[4:]
		request.done(nil)
		if log.IsLevelEnabled(log.TraceLevel) {
			log.WithField("requestID", id).WithField("action", "done").Trace("recv")
		}
	}
}

func (r *req) nextID() uint32 {
	return atomic.AddUint32(&r.reqID, 1)
}

func (r *req) Request(content []byte) ([]byte, error) {
	return r.ReqeustTimeout(r.timeout, content)
}

func (r *req) ReqeustTimeout(t time.Duration, content []byte) ([]byte, error) {
	request := r.ReqeustAsync(content)
	select {
	case request = <-r.ReqeustAsync(content).Done:
		return request.Reply, request.Err
	case <-time.After(t):
		request.Cancel()
		return nil, multisocket.ErrTimeout
	}
}

func (r *req) ReqeustAsync(content []byte) *Request {
	request := &Request{
		r:    r,
		ID:   r.nextID(),
		Done: make(chan *Request, 2),
	}
	r.Lock()
	r.requests[request.ID] = request
	r.Unlock()
	if log.IsLevelEnabled(log.TraceLevel) {
		log.WithField("requestID", request.ID).
			WithField("action", "start").Trace("request")
	}

	idBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(idBuf, request.ID)
	if err := r.Send(idBuf, content); err != nil {
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithError(err).WithField("requestID", request.ID).Debug("request")
		}
		request.Cancel()
		request.done(err)
	}
	return request
}

func (req *Request) done(err error) {
	select {
	case req.Done <- req:
		req.Err = err
	default:
		// request success before cancel
	}
}

// Cancel cancel this request
func (req *Request) Cancel() {
	r := req.r
	r.Lock()
	delete(r.requests, req.ID)
	r.Unlock()
}

func (r *req) Close() {
	r.Lock()
	defer r.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	r.Socket.Close()
}
