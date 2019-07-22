package test

import (
	"math/rand"
	"time"

	"github.com/multisocket/multisocket"
	"github.com/multisocket/multisocket/address"
	"github.com/multisocket/multisocket/options"
)

var (
	transports = []struct {
		name string
		addr string
	}{
		{"inproc.channel", "inproc.channel://benchmark_test"},
		{"inproc.iopipe", "inproc.iopipe://benchmark_test"},
		{"inproc.netpipe", "inproc.netpipe://benchmark_test"},
		{"ipc", "ipc:///tmp/benchmark_test.sock"},
		{"tcp", "tcp://127.0.0.1:33833"},
		{"ws", "ws://127.0.0.1:44844/ws"},
	}

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
		{"128KB", 128 * 1024},
	}
)

func init() {
	initRandSeed(0)
}

func initRandSeed(randSeed int64) int64 {
	if randSeed == 0 {
		randSeed = time.Now().UnixNano()
	}
	rand.Seed(randSeed)
	return randSeed
}

func prepareSocks(addr string, ovses ...options.OptionValues) (srvsock, clisock multisocket.Socket, err error) {
	var sa address.MultiSocketAddress

	if sa, err = address.ParseMultiSocketAddress(addr); err != nil {
		return
	}

	srvsock = multisocket.New(nil)
	clisock = multisocket.New(nil)
	if err = sa.Listen(srvsock, ovses...); err != nil {
		return
	}
	if err = sa.Dial(clisock, ovses...); err != nil {
		return
	}
	return
}

func genRandomContent(sz int) (b []byte) {
	b = make([]byte, sz)
	rand.Read(b)
	return
}
