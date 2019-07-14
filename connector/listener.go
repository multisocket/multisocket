package connector

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/transport"
)

type listener struct {
	options.Options

	parent *connector
	addr   string
	transport.Listener
	sync.Mutex
	closed bool

	stopped bool
}

func newListener(parent *connector, addr string, tl transport.Listener, ovs options.OptionValues) *listener {
	return &listener{
		Options:  options.NewOptionsWithValues(ovs),
		parent:   parent,
		addr:     addr,
		Listener: tl,
		closed:   false,
	}
}

func (l *listener) start() {
	l.Lock()
	defer l.Unlock()
	if !l.stopped {
		return
	}

	l.stopped = false
}

func (l *listener) stop() {
	l.Lock()
	defer l.Unlock()
	if l.stopped {
		return
	}

	l.stopped = true
}

func (l *listener) isStopped() bool {
	l.Lock()
	defer l.Unlock()
	return l.stopped
}

// serve spins in a loop, calling the accepter's Accept routine.
func (l *listener) serve() {
	if log.IsLevelEnabled(log.DebugLevel) {
		raw := Options.Pipe.Raw.ValueFrom(l.Options)
		log.WithFields(log.Fields{"addr": l.addr, "action": "start", "raw": raw}).Debug("accept")
	}
	for {
		// If the underlying PipeListener is closed, or not
		// listening, we expect to return back with an error.
		if tc, err := l.Listener.Accept(l.Options); err == errs.ErrClosed {
			break
		} else if err == nil {
			if l.isStopped() {
				tc.Close()
			} else {
				go l.parent.addPipe(newPipe(l.parent, tc, nil, l, l.Options))
			}
		} else {
			// Debounce a little bit, to avoid thrashing the CPU.
			time.Sleep(time.Second / 100)
		}
	}
	if log.IsLevelEnabled(log.DebugLevel) {
		raw := Options.Pipe.Raw.ValueFrom(l.Options)
		log.WithFields(log.Fields{"addr": l.addr, "action": "end", "raw": raw}).Debug("accept")
	}
}

func (l *listener) Listen() error {
	if err := l.Listener.Listen(l.Options); err != nil {
		return err
	}

	go l.serve()
	return nil
}

func (l *listener) Close() error {
	l.Lock()
	defer l.Unlock()
	if l.closed {
		return errs.ErrClosed
	}
	l.closed = true
	return l.Listener.Close()
}

func (l *listener) TransportListener() transport.Listener {
	return l.Listener
}
