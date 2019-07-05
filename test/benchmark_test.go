package test

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/webee/multisocket"
	"github.com/webee/multisocket/message"
	_ "github.com/webee/multisocket/transport/all"
)

var (
	sizes = []struct {
		name string
		sz   int
	}{
		{"0B", 0},
		{"64B", 128},
		{"128B", 128},
		{"256B", 256},
		{"512B", 512},
		{"1KB", 1024},
		{"2KB", 2 * 1024},
		{"4KB", 4 * 1024},
		{"8KB", 8 * 1024},
		{"16KB", 16 * 1024},
		{"32KB", 32 * 1024},
		{"64KB", 64 * 1024},
	}

	transports = []struct {
		name string
		addr string
	}{
		{"inproc", "inproc://benchmark_test"},
		{"ipc", "ipc:///tmp/benchmark_test.sock"},
		{"tcp", "tcp://127.0.0.1:33833"},
	}
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	os.Exit(m.Run())
}

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
	srvsock := multisocket.NewDefault()
	defer srvsock.Close()

	clisock := multisocket.NewDefault()
	defer clisock.Close()

	if err := srvsock.Listen(addr); err != nil {
		b.Errorf("server listen error: %s", err)
		return
	}

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
			done <- struct{}{}
			msg.FreeAll()
		}
	}()

	if err := clisock.Dial(addr); err != nil {
		b.Errorf("client dial error: %s", err)
		return
	}

	time.Sleep(500 * time.Millisecond)

	var (
		err     error
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

// benchmark a group of messages' total average latency
func benchmarkGroupLatency(b *testing.B, addr string, sz int) {
	srvsock := multisocket.NewDefault()
	defer srvsock.Close()

	clisock := multisocket.NewDefault()
	defer clisock.Close()

	if err := srvsock.Listen(addr); err != nil {
		b.Errorf("server listen error: %s", err)
		return
	}

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

	if err := clisock.Dial(addr); err != nil {
		b.Errorf("client dial error: %s", err)
		return
	}

	time.Sleep(500 * time.Millisecond)

	var (
		err     error
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
	srvsock := multisocket.NewDefault()
	defer srvsock.Close()

	clisock := multisocket.NewDefault()
	defer clisock.Close()

	if err := srvsock.Listen(addr); err != nil {
		b.Errorf("server listen error: %s", err)
		return
	}

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

	if err := clisock.Dial(addr); err != nil {
		b.Errorf("client dial error: %s", err)
		return
	}

	time.Sleep(500 * time.Millisecond)
	// xx MB/s => xx M(msg)/s
	b.SetBytes(1)

	b.ResetTimer()
	var (
		err     error
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
	srvsock := multisocket.NewDefault()
	defer srvsock.Close()

	clisock := multisocket.NewDefault()
	defer clisock.Close()

	if err := srvsock.Listen(addr); err != nil {
		b.Errorf("server listen error: %s", err)
		return
	}

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

	if err := clisock.Dial(addr); err != nil {
		b.Errorf("client dial error: %s", err)
		return
	}

	time.Sleep(500 * time.Millisecond)
	// xx MB/s => xx M(msg)/s
	b.SetBytes(1)

	b.ResetTimer()
	var (
		err error
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
