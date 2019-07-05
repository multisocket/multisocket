package utils

import (
	"bufio"
	"time"
)

type (
	// AutoFlushBufferWriter auto flush buffer writer
	AutoFlushBufferWriter struct {
		*bufio.Writer
		flushTimer *time.Timer
		period     time.Duration
	}
)

const (
	defaultFlushPeriod = 10 * time.Millisecond
)

// NewAutoFlushBufferWriter create an AutoFlushBufferWriter with default flush period
func NewAutoFlushBufferWriter(w *bufio.Writer, period time.Duration) *AutoFlushBufferWriter {
	return NewAutoFlushBufferWriterWithPeriod(w, defaultFlushPeriod)
}

// NewAutoFlushBufferWriterWithPeriod create an AutoFlushBufferWriter with flush period
func NewAutoFlushBufferWriterWithPeriod(w *bufio.Writer, period time.Duration) *AutoFlushBufferWriter {
	return &AutoFlushBufferWriter{
		Writer:     w,
		flushTimer: time.AfterFunc(1024*time.Hour, func() { w.Flush() }),
		period:     period,
	}
}

func (w *AutoFlushBufferWriter) Write(p []byte) (n int, err error) {
	if !w.flushTimer.Stop() {
		select {
		case <-w.flushTimer.C:
		default:
		}
	}
	n, err = w.Writer.Write(p)
	w.flushTimer.Reset(w.period)
	return
}
