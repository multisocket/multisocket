package bytespool

import (
	"sync"
)

type (
	poolInfo struct {
		sz int
		p  *sync.Pool
	}
)

func newPoolInfo(sz int) *poolInfo {
	return &poolInfo{
		sz: sz,
		p: &sync.Pool{New: func() interface{} {
			return make([]byte, 0, sz)
		}},
	}
}

var (
	pools = []*poolInfo{
		newPoolInfo(4),
		newPoolInfo(8),
		newPoolInfo(16),
		newPoolInfo(32),
		newPoolInfo(64),
		newPoolInfo(128),
		newPoolInfo(256),
		newPoolInfo(512),
		newPoolInfo(1024),
		newPoolInfo(2 * 1024),
		newPoolInfo(4 * 1024),
		newPoolInfo(8 * 1024),
		newPoolInfo(16 * 1024),
	}
	extraPools = []*poolInfo{}
)

func init() {
	// 16KB as the increment unit
	for i := 2; i <= 64; i++ {
		pools = append(pools, newPoolInfo(i*8*1024))
	}
}

// Alloc alloc bytes
func Alloc(sz int) []byte {
	if sz <= 0 {
		return nil
	}

	for _, pi := range pools {
		if sz <= pi.sz {
			// to requested size.
			return pi.p.Get().([]byte)[:sz]
		}
	}
	return make([]byte, sz)
}

// Free bytes
func Free(p []byte) {
	sz := cap(p)
	if sz <= 0 {
		return
	}
	for _, pi := range pools {
		if sz == pi.sz {
			pi.p.Put(p)
		}
	}
}
