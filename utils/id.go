package utils

import (
	"math/rand"
	"sync"
	"time"
)

// RecyclableIDGenerator generate recyclable unique ids.
type RecyclableIDGenerator struct {
	sync.Mutex
	ids  map[uint32]struct{}
	next uint32
}

// NewRecyclableIDGenerator create an id generator
func NewRecyclableIDGenerator() *RecyclableIDGenerator {
	return &RecyclableIDGenerator{
		ids:  make(map[uint32]struct{}),
		next: uint32(rand.NewSource(time.Now().UnixNano()).Int63()),
	}
}

// NextID get the next id
func (g *RecyclableIDGenerator) NextID() (id uint32) {
	g.Lock()
	defer g.Unlock()
	for {
		id = g.next
		g.next++
		if id == 0 {
			continue
		}
		if _, ok := g.ids[id]; !ok {
			g.ids[id] = struct{}{}
			break
		}
	}
	return
}

// Recycle recyle the id for future use.
func (g *RecyclableIDGenerator) Recycle(id uint32) {
	g.Lock()
	delete(g.ids, id)
	g.Unlock()
}
