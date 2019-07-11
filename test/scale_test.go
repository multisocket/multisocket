package test

// This simple test just fires off a crapton of inproc clients, to see
// how many connections we could handle.  We do this using inproc, because
// we would absolutely exhaust TCP ports before we would hit any of the
// natural limits.  The inproc transport has no such limits, so we are
// effectively just testing goroutine scalability, which is what we want.
// The intention is to demonstrate that multisocket can address the C10K problem
// without breaking a sweat.

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/webee/multisocket"
	"github.com/webee/multisocket/message"
	_ "github.com/webee/multisocket/transport/inproc"
)

func scalabilityClient(t *testing.T, idx int, errp *error, loops int, wg, connWg *sync.WaitGroup) {
	defer wg.Done()
	sock := multisocket.NewDefault()
	defer sock.Close()
	if err := sock.Dial("inproc://atscale"); err != nil {
		*errp = err
		return
	}
	// t.Logf("CONNECTED: %d", idx)
	connWg.Done()
	connWg.Wait()

	var (
		err error
		msg *message.Message
	)
	for i := 0; i < loops; i++ {
		// Inject a random sleep to avoid pounding the CPU too hard.
		time.Sleep(time.Duration(rand.Int31n(1000)) * time.Microsecond)

		if err = sock.Send([]byte("ping")); err != nil {
			*errp = err
			return
		}

		if msg, err = sock.RecvMsg(); err != nil {
			*errp = err
			return
		}
		if string(msg.Content) != "pong" {
			*errp = fmt.Errorf("response mismatch: %d/%d", i, idx)
			return
		}
		msg.FreeAll()
		// t.Logf("OK: %d/%d", i, idx)
	}
	// clean initial error
	*errp = nil
}

func scalabilityServer(t *testing.T, sock multisocket.Socket) {
	defer sock.Close()
	var (
		err error
		msg *message.Message
	)
	for {
		if msg, err = sock.RecvMsg(); err != nil {
			return
		}
		if err = sock.SendTo(msg.Source, []byte("pong")); err != nil {
			return
		}
		msg.FreeAll()
	}
}

func TestScalability(t *testing.T) {
	// Beyond this things get crazy.
	// Note that the thread count actually indicates that you will
	// have this many client sockets, and an equal number of server
	// side pipes.
	loops := 10
	threads := 20000

	initialErr := errors.New("initial")
	errs := make([]error, threads)

	ssock := multisocket.NewDefault()
	if e := ssock.Listen("inproc://atscale"); e != nil {
		t.Fatalf("Cannot listen: %v", e)
	}
	time.Sleep(time.Millisecond * 100)
	go scalabilityServer(t, ssock)

	connWg := &sync.WaitGroup{}
	connWg.Add(threads)
	wg := &sync.WaitGroup{}
	wg.Add(threads)
	for i := 0; i < threads; i++ {
		errs[i] = initialErr
		go scalabilityClient(t, i, &errs[i], loops, wg, connWg)
	}

	wg.Wait()
	for i := 0; i < threads; i++ {
		if errs[i] != nil {
			t.Fatalf("Test failed: %v", errs[i])
		}
	}
}
