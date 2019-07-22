package test

import (
	"sync"
	"testing"
	"time"

	"github.com/multisocket/multisocket"
	"github.com/multisocket/multisocket/message"
	_ "github.com/multisocket/multisocket/transport/all"
)

func BenchmarkSingleLatency(b *testing.B) {
	for idx := range sizes {
		size := sizes[idx]
		b.Run(size.name, func(b *testing.B) {
			sz := size.sz
			for idx := range transports {
				tp := transports[idx]
				b.Run(tp.name, func(b *testing.B) {
					addr := tp.addr
					benchmarkSingleLatency(b, addr, sz)
				})
			}
		})
	}
}

func BenchmarkRoundLatency(b *testing.B) {
	for idx := range sizes {
		size := sizes[idx]
		b.Run(size.name, func(b *testing.B) {
			sz := size.sz
			for idx := range transports {
				tp := transports[idx]
				b.Run(tp.name, func(b *testing.B) {
					addr := tp.addr
					benchmarkRoundLatency(b, addr, sz)
				})
			}
		})
	}
}

func BenchmarkGroupLatency(b *testing.B) {
	for idx := range sizes {
		size := sizes[idx]
		b.Run(size.name, func(b *testing.B) {
			sz := size.sz
			for idx := range transports {
				tp := transports[idx]
				b.Run(tp.name, func(b *testing.B) {
					addr := tp.addr
					benchmarkGroupLatency(b, addr, sz)
				})
			}
		})
	}
}

func BenchmarkSendThroughput(b *testing.B) {
	for idx := range sizes {
		size := sizes[idx]
		b.Run(size.name, func(b *testing.B) {
			sz := size.sz
			for idx := range transports {
				tp := transports[idx]
				b.Run(tp.name, func(b *testing.B) {
					addr := tp.addr
					benchmarkSendThroughput(b, addr, sz)
				})
			}
		})
	}
}

func BenchmarkRecvThroughput(b *testing.B) {
	for idx := range sizes {
		size := sizes[idx]
		b.Run(size.name, func(b *testing.B) {
			sz := size.sz
			for idx := range transports {
				tp := transports[idx]
				b.Run(tp.name, func(b *testing.B) {
					addr := tp.addr
					benchmarkRecvThroughput(b, addr, sz)
				})
			}
		})
	}
}

// benchmark single message's average latency
func benchmarkSingleLatency(b *testing.B, addr string, sz int) {
	var (
		err     error
		srvsock multisocket.Socket
		clisock multisocket.Socket
	)
	if srvsock, clisock, err = prepareSocks(addr); err != nil {
		b.Errorf("connect error: %s", err)
	}
	defer srvsock.Close()
	defer clisock.Close()

	done := make(chan struct{}, 1)
	go func() {
		var (
			err error
			msg *message.Message
		)
		for {
			if msg, err = srvsock.RecvMsg(); err != nil {
				return
			}
			b.StopTimer()
			msg.FreeAll()
			done <- struct{}{}
		}
	}()

	time.Sleep(500 * time.Millisecond)

	var (
		content = make([]byte, sz)
	)
	b.SetBytes(1)
	b.ResetTimer()
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		if err = clisock.Send(content); err != nil {
			b.Errorf("client send error: %s", err)
			return
		}
		<-done
	}

	b.StopTimer()
}

// benchmark message's send/recv round latency
func benchmarkRoundLatency(b *testing.B, addr string, sz int) {
	var (
		err     error
		srvsock multisocket.Socket
		clisock multisocket.Socket
	)
	if srvsock, clisock, err = prepareSocks(addr); err != nil {
		b.Errorf("connect error: %s", err)
	}
	defer srvsock.Close()
	defer clisock.Close()

	var content = make([]byte, sz)
	go func() {
		var (
			err error
			msg *message.Message
		)
		for {
			if msg, err = srvsock.RecvMsg(); err != nil {
				return
			}
			if err = srvsock.SendTo(msg.Source, content); err != nil {
				return
			}
			msg.FreeAll()
		}
	}()

	time.Sleep(500 * time.Millisecond)

	var (
		msg *message.Message
	)
	b.SetBytes(1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err = clisock.Send(content); err != nil {
			b.Errorf("client send error: %s", err)
			return
		}
		if msg, err = clisock.RecvMsg(); err != nil {
			b.Errorf("client recv error: %s", err)
			return
		}
		msg.FreeAll()
	}

	b.StopTimer()
}

// benchmark a group of messages' total average latency
func benchmarkGroupLatency(b *testing.B, addr string, sz int) {
	var (
		err     error
		srvsock multisocket.Socket
		clisock multisocket.Socket
	)
	if srvsock, clisock, err = prepareSocks(addr); err != nil {
		b.Errorf("connect error: %s", err)
	}
	defer srvsock.Close()
	defer clisock.Close()

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		var (
			err error
			msg *message.Message
		)
		for i := 0; i < b.N; i++ {
			if msg, err = srvsock.RecvMsg(); err != nil {
				break
			}
			msg.FreeAll()
		}
		wg.Done()
	}()

	time.Sleep(500 * time.Millisecond)

	var (
		content = make([]byte, sz)
	)
	b.SetBytes(1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err = clisock.Send(content); err != nil {
			b.Errorf("client send error: %s", err)
			return
		}
	}
	wg.Wait()

	b.StopTimer()
}

// benchmark sender side's throughput, use -benchmem to see xx MB/s => xx M(msg)/s
func benchmarkSendThroughput(b *testing.B, addr string, sz int) {
	var (
		err     error
		srvsock multisocket.Socket
		clisock multisocket.Socket
	)
	if srvsock, clisock, err = prepareSocks(addr); err != nil {
		b.Errorf("connect error: %s", err)
	}
	defer srvsock.Close()
	defer clisock.Close()

	go func() {
		// just recv content
		for {
			msg, err := srvsock.RecvMsg()
			if err != nil {
				break
			}
			msg.FreeAll()
		}
	}()

	time.Sleep(500 * time.Millisecond)
	// xx MB/s => xx M(msg)/s
	b.SetBytes(1)

	b.ResetTimer()
	var (
		content = make([]byte, sz)
	)
	for i := 0; i < b.N; i++ {
		if err = clisock.Send(content); err != nil {
			b.Errorf("client send error: %s", err)
			return
		}
	}

	b.StopTimer()
}

// benchmark receiver side's throughput, use -benchmem to see xx MB/s => xx M(msg)/s
func benchmarkRecvThroughput(b *testing.B, addr string, sz int) {
	var (
		err     error
		srvsock multisocket.Socket
		clisock multisocket.Socket
	)
	if srvsock, clisock, err = prepareSocks(addr); err != nil {
		b.Errorf("connect error: %s", err)
	}
	defer srvsock.Close()
	defer clisock.Close()

	go func() {
		var (
			err     error
			content = make([]byte, sz)
		)
		// just send content
		for {
			if err = srvsock.Send(content); err != nil {
				return
			}
		}
	}()

	time.Sleep(500 * time.Millisecond)
	// xx MB/s => xx M(msg)/s
	b.SetBytes(1)

	b.ResetTimer()
	var (
		msg *message.Message
	)
	for i := 0; i < b.N; i++ {
		if msg, err = clisock.RecvMsg(); err != nil {
			b.Errorf("client recv error: %s", err)
			return
		}
		msg.FreeAll()
	}

	b.StopTimer()
}
