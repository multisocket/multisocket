package connector

import (
	"math/rand"
	"sync"
	"time"

	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/transport"
)

const (
	defaultMinReconnTime = time.Millisecond * 100
	defaultMaxReconnTime = time.Second * 8
	defaultDialAsync     = false
)

type dialer struct {
	options.Options
	parent *connector
	d      transport.Dialer

	sync.Mutex
	closed     bool
	stopped    bool
	active     bool
	dialing    bool
	connected  bool
	redialer   *time.Timer
	reconnTime time.Duration
}

func newDialer(parent *connector, td transport.Dialer) *dialer {
	opts := options.NewOptionsWithUpDownStreamsAndAccepts(nil, td,
		DialerOptionMinReconnectTime,
		DialerOptionMaxReconnectTime,
		DialerOptionDialAsync)
	return &dialer{
		Options: opts,
		parent:  parent,
		d:       td,
	}
}

//options
func (d *dialer) minReconnectTime() time.Duration {
	return DialerOptionMinReconnectTime.Value(d.GetOptionDefault(DialerOptionMinReconnectTime, defaultMinReconnTime))
}

func (d *dialer) maxReconnectTime() time.Duration {
	return DialerOptionMaxReconnectTime.Value(d.GetOptionDefault(DialerOptionMaxReconnectTime, defaultMaxReconnTime))
}

func (d *dialer) dialAsync() bool {
	return DialerOptionDialAsync.Value(d.GetOptionDefault(DialerOptionDialAsync, defaultDialAsync))
}

func (d *dialer) Dial() error {
	d.Lock()
	if d.active {
		d.Unlock()
		return ErrAddrInUse
	}
	if d.closed {
		d.Unlock()
		return ErrClosed
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
	defer d.Unlock()
	if d.closed {
		return ErrClosed
	}
	d.closed = true
	return nil
}

func (d *dialer) start() {
	d.Lock()
	defer d.Unlock()
	if !d.stopped {
		return
	}

	d.stopped = false
	time.AfterFunc(d.reconnTime, d.redial)
}

func (d *dialer) stop() {
	d.Lock()
	defer d.Unlock()
	if d.stopped {
		return
	}

	d.stopped = true
}

func (d *dialer) pipeClosed() {
	// We always want to sleep a little bit after the pipe closed down,
	// to avoid spinning hard.  This can happen if we connect, but the
	// peer refuses to accept our protocol.  Injecting at least a little
	// delay should help.
	d.Lock()
	d.connected = false
	time.AfterFunc(d.reconnTime, d.redial)
	d.Unlock()
}

func (d *dialer) dial(redial bool) error {
	d.Lock()
	if d.stopped {
		return nil
	}

	if d.dialing || d.connected || d.closed {
		// If we already have a dial in progress, then stop.
		// This really should never occur (see comments below),
		// but having multiple dialers create multiple pipes is
		// probably bad.  So be paranoid -- I mean "defensive" --
		// for now.
		d.Unlock()
		return ErrAddrInUse
	}
	if d.redialer != nil {
		d.redialer.Stop()
	}
	d.dialing = true
	d.Unlock()

	tc, err := d.d.Dial()
	if err == nil {
		d.parent.addPipe(newPipe(d.parent, tc, d, nil))

		d.Lock()
		d.dialing = false
		d.connected = true
		d.reconnTime = d.minReconnectTime()
		d.Unlock()
		return nil
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
