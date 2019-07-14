package reqrep

import (
	"bytes"
	"encoding/binary"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket"
	"github.com/webee/multisocket/errs"
	"github.com/webee/multisocket/sender"
)

type (
	req struct {
		multisocket.Socket
		timeout time.Duration

		sync.RWMutex
		closedq  chan struct{}
		reqID    uint32
		requests map[uint32]*Request
	}
)

const (
	defaultTimeout = time.Second * 16
)

// NewReq create a Req protocol instance
func NewReq() Req {
	return NewReqWithTimeout(defaultTimeout)
}

// NewReqWithTimeout create a Req protocol instance with request timeout
func NewReqWithTimeout(timeout time.Duration) Req {
	sock := multisocket.NewDefault()
	sock.GetSender().SetOption(sender.Options.SendBestEffort, true)

	req := &req{
		Socket:   sock,
		timeout:  timeout,
		closedq:  make(chan struct{}),
		reqID:    uint32(time.Now().UnixNano()), // quasi-random
		requests: make(map[uint32]*Request),
	}

	// TODO: dynamic adjust worker
	go req.run()
	return req
}

func (r *req) GetSocket() multisocket.Socket {
	return r.Socket
}

func (r *req) run() {
	var (
		err     error
		ok      bool
		content []byte
		request *Request
	)
	for {
		if content, err = r.Recv(); err != nil {
			break
		}
		requestID := binary.BigEndian.Uint32(content)
		r.RLock()
		if request, ok = r.requests[requestID]; !ok {
			r.RUnlock()
			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithField("requestID", requestID).WithField("action", "miss").Warn("recv")
			}
			continue
		}
		r.RUnlock()
		r.Lock()
		delete(r.requests, requestID)
		r.Unlock()

		request.Reply = content[4:]
		request.done(nil)
	}
}

func (r *req) nextID() uint32 {
	return atomic.AddUint32(&r.reqID, 1)
}

func (r *req) Request(content []byte) ([]byte, error) {
	return r.ReqeustTimeout(r.timeout, content)
}

func (r *req) ReqeustTimeout(t time.Duration, content []byte) (res []byte, err error) {
	var (
		tm *time.Timer
		tq <-chan time.Time
	)
	if t > 0 {
		tm = time.NewTimer(t)
		tq = tm.C
	}

	request := r.ReqeustAsync(content)
	select {
	case <-r.closedq:
		err = errs.ErrClosed
	case request = <-request.Done:
		res = request.Reply
		err = request.Err
	case <-tq:
		request.Cancel()
		err = errs.ErrTimeout
		return
	}
	if tm != nil {
		tm.Stop()
	}
	return
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

	buf := bytesBufferPool.Get().(*bytes.Buffer)
	binary.Write(buf, binary.BigEndian, requestID)
	buf.Write(content)
	if err := r.Send(buf.Bytes()); err != nil {
		request.Cancel()
		request.done(err)
	}
	buf.Reset()
	bytesBufferPool.Put(buf)
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
	select {
	case <-r.closedq:
		r.Unlock()
		return errs.ErrClosed
	default:
		close(r.closedq)
	}
	r.Unlock()

	return r.Socket.Close()
}
