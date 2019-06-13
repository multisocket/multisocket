package reqrep

import (
	"encoding/binary"
	"sync"
	"sync/atomic"
	"time"

	"github.com/webee/multisocket/options"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket"
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/receiver"
	"github.com/webee/multisocket/sender"
)

type (
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

// NewReq create a Req protocol instance
func NewReq() Req {
	return NewReqWithTimeout(defaultTimeout)
}

// NewReqWithTimeout create a Req protocol instance with request timeout
func NewReqWithTimeout(timeout time.Duration) Req {
	req := &req{
		Socket: multisocket.New(connector.New(),
			sender.NewWithOptions(options.NewOptionValue(sender.OptionSendBestEffort, true)),
			receiver.New()),
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
		requestID := binary.BigEndian.Uint32(msg.Content)
		if log.IsLevelEnabled(log.TraceLevel) {
			log.WithField("requestID", requestID).WithField("action", "start").Trace("recv")
		}
		r.Lock()
		if request, ok = r.requests[requestID]; !ok {
			r.Unlock()
			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithField("requestID", requestID).WithField("action", "miss").Debug("recv")
			}
			continue
		}
		delete(r.requests, requestID)
		r.Unlock()

		request.Reply = msg.Content[4:]
		request.done(nil)
		if log.IsLevelEnabled(log.TraceLevel) {
			log.WithField("requestID", requestID).WithField("action", "done").Trace("recv")
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
	case request = <-request.Done:
		return request.Reply, request.Err
	case <-time.After(t):
		request.Cancel()
		return nil, multisocket.ErrTimeout
	}
}

func (r *req) ReqeustAsync(content []byte) *Request {
	requestID := r.nextID()
	request := &Request{
		Cancel: r.cancelRequest(requestID),
		Done:   make(chan *Request, 2),
	}
	r.Lock()
	r.requests[requestID] = request
	r.Unlock()
	if log.IsLevelEnabled(log.TraceLevel) {
		log.WithField("requestID", requestID).
			WithField("action", "start").Trace("request")
	}

	idBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(idBuf, requestID)
	if err := r.Send(idBuf, content); err != nil {
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithError(err).WithField("requestID", requestID).Debug("request")
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

func (r *req) cancelRequest(requestID uint32) func() {
	// Cancel cancel this request
	return func() {
		r.Lock()
		delete(r.requests, requestID)
		r.Unlock()
	}
}

func (r *req) Close() error {
	r.Lock()
	defer r.Unlock()
	if r.closed {
		return multisocket.ErrClosed
	}
	r.closed = true
	return r.Socket.Close()
}
