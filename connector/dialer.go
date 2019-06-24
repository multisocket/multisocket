package connector

import (
	"math/rand"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/errs"
	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/transport"
)

type dialer struct {
	options.Options
	parent *connector
	addr   string
	d      transport.Dialer

	sync.Mutex
	closedq    chan struct{}
	stopped    bool
	active     bool
	dialing    bool
	connected  bool
	redialer   *time.Timer
	reconnTime time.Duration
}

func newDialer(parent *connector, addr string, td transport.Dialer, ovs options.OptionValues) *dialer {
	return &dialer{
		Options: options.NewOptionsWithValues(ovs),
		parent:  parent,
		addr:    addr,
		d:       td,
		closedq: make(chan struct{}),
	}
}

//options
func (d *dialer) minReconnectTime() time.Duration {
	return Options.Dialer.MinReconnectTime.ValueFrom(d)
}

func (d *dialer) maxReconnectTime() time.Duration {
	return Options.Dialer.MaxReconnectTime.ValueFrom(d)
}

func (d *dialer) dialAsync() bool {
	return Options.Dialer.DialAsync.ValueFrom(d)
}

func (d *dialer) reconnect() bool {
	return Options.Dialer.Reconnect.ValueFrom(d)
}

func (d *dialer) Dial() error {
	select {
	case <-d.closedq:
		return errs.ErrClosed
	default:
	}
	d.Lock()
	if d.active {
		d.Unlock()
		return errs.ErrAddrInUse
	}

	d.active = true
	d.reconnTime = d.minReconnectTime()
	d.Unlock()
	async := d.dialAsync()
	if async {
		go d.redial()
		return nil
	}
	return d.dial(false)
}

func (d *dialer) Close() error {
	d.Lock()
	select {
	case <-d.closedq:
		d.Unlock()
		return errs.ErrClosed
	default:
		close(d.closedq)
	}
	d.Unlock()
	return nil
}

func (d *dialer) start() {
	d.Lock()
	if !d.stopped {
		d.Unlock()
		return
	}
	d.stopped = false
	d.Unlock()

	d.reconn()
}

func (d *dialer) stop() {
	d.Lock()
	if d.stopped {
		d.Unlock()
		return
	}

	d.stopped = true
	if d.redialer != nil {
		d.redialer.Stop()
	}
	d.Unlock()
}

func (d *dialer) reconn() bool {
	select {
	case <-d.closedq:
		return true
	default:
	}

	if !d.reconnect() {
		return false
	}

	d.Lock()
	if d.redialer != nil {
		d.redialer.Stop()
	}
	d.redialer = time.AfterFunc(d.reconnTime, d.redial)
	d.Unlock()
	return true
}

func (d *dialer) pipeClosed() {
	d.Lock()
	d.connected = false
	d.Unlock()

	if !d.reconn() {
		d.parent.remDialer(d)
	}
}

func (d *dialer) dial(redial bool) error {
	select {
	case <-d.closedq:
		return errs.ErrClosed
	default:
	}

	d.Lock()
	if d.stopped {
		d.Unlock()
		return ErrStopped
	}

	if d.dialing || d.connected {
		d.Unlock()
		return errs.ErrAddrInUse
	}
	if d.redialer != nil {
		d.redialer.Stop()
		d.redialer = nil
	}
	d.dialing = true
	d.Unlock()

	if log.IsLevelEnabled(log.DebugLevel) {
		raw := transport.Options.RawMode.ValueFrom(d.Options)
		log.WithFields(log.Fields{"addr": d.addr, "action": "start", "raw": raw}).Debug("dial")
	}
	tc, err := d.d.Dial(d.Options)
	if err == nil {
		if log.IsLevelEnabled(log.DebugLevel) {
			raw := transport.Options.RawMode.ValueFrom(d.Options)
			log.WithFields(log.Fields{"addr": d.addr, "action": "success", "raw": raw}).Debug("dial")
		}
		d.parent.addPipe(newPipe(d.parent, tc, d, nil, d.Options))

		d.Lock()
		d.dialing = false
		d.connected = true
		d.reconnTime = d.minReconnectTime()
		d.Unlock()
		return nil
	}
	if log.IsLevelEnabled(log.DebugLevel) {
		raw := transport.Options.RawMode.ValueFrom(d.Options)
		log.WithError(err).WithFields(log.Fields{"addr": d.addr, "action": "failed", "raw": raw}).Error("dial")
	}

	d.Lock()
	defer d.Unlock()
	// We're no longer dialing, so let another reschedule happen, if
	// appropriate.   This is quite possibly paranoia.  We should only
	// be in this routine in the following circumstances:
	//
	// 1. Initial dialing (via Dial())
	// 2. After a previously created pipe fails and is closed due to error.
	// 3. After timing out from a failed connection attempt.
	//
	// The above cases should be mutually exclusive.  But paranoia.
	// Consider removing the d.dialing logic later if we can prove
	// that this never occurs.
	d.dialing = false

	if !redial {
		return err
	}

	// Exponential backoff, and jitter.  Our backoff grows at
	// about 1.3x on average, so we don't penalize a failed
	// connection too badly.
	minfact := float64(1.1)
	maxfact := float64(1.5)
	actfact := rand.Float64()*(maxfact-minfact) + minfact
	rtime := d.reconnTime
	d.reconnTime = time.Duration(actfact * float64(d.reconnTime))
	reconnMaxTime := d.maxReconnectTime()
	if reconnMaxTime != 0 {
		if d.reconnTime > reconnMaxTime {
			d.reconnTime = reconnMaxTime
		}
	}
	d.redialer = time.AfterFunc(rtime, d.redial)
	return err
}

func (d *dialer) redial() {
	d.dial(true)
}
