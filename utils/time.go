package utils

import (
	"time"
)

type (
	// Timer wraps the time.Timer
	Timer struct {
		C  <-chan time.Time
		tm *time.Timer
	}
)

// NewTimer create a timer
func NewTimer() *Timer {
	return new(Timer)
}

func (t *Timer) stop() {
	if !t.tm.Stop() {
		select {
		case <-t.tm.C:
		default:
		}
	}
}

// Stop prevents the Timer from firing.
func (t *Timer) Stop() {
	if t.tm == nil {
		return
	}
	t.stop()
	t.C = nil
}

// Reset reset the timer to next duration
func (t *Timer) Reset(d time.Duration) {
	if t.tm == nil {
		t.tm = time.NewTimer(d)
	} else {
		t.stop()
		t.tm.Reset(d)
	}
	t.C = t.tm.C
}
