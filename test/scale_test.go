package test

// This simple test just fires off a crapton of inproc clients, to see
// how many connections we could handle.  We do this using inproc, because
// we would absolutely exhaust TCP ports before we would hit any of the
// natural limits.  The inproc transport has no such limits, so we are
// effectively just testing goroutine scalability, which is what we want.
// The intention is to demonstrate that multisocket can address the C10K problem
// without breaking a sweat.

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/multisocket/multisocket"
	"github.com/multisocket/multisocket/message"
	_ "github.com/multisocket/multisocket/transport/inproc"
)

func TestScalabilityInprocSendReply(t *testing.T) {
	testScalabilitySendReply(t, "inproc://atscale_send_reply", 10, 20000)
}

func TestScalabilityInprocSendAll(t *testing.T) {
	testScalabilitySendAll(t, "inproc://atscale_send_all", 10, 20000)
}

func testScalabilitySendReply(t *testing.T, addr string, loops int, threads int) {
	// Beyond this things get crazy.
	// Note that the thread count actually indicates that you will
	// have this many client sockets, and an equal number of server
	// side pipes.
	initialErr := errors.New("initial")
	errs := make([]error, threads)

	ssock := multisocket.New(nil)
	if e := ssock.Listen(addr); e != nil {
		t.Fatalf("Cannot listen: %v", e)
	}
	go scalabilitySendReplyServer(t, ssock)

	connWg := &sync.WaitGroup{}
	connWg.Add(threads)
	wg := &sync.WaitGroup{}
	wg.Add(threads)
	connStartAt := time.Now()
	for i := 0; i < threads; i++ {
		errs[i] = initialErr
		go scalabilitySendReplyClient(t, i, addr, &errs[i], loops, wg, connWg)
	}
	// all clients are connected
	connWg.Wait()
	// fmt.Fprintf(os.Stderr, "%d clients connected in %s\n", threads, time.Now().Sub(connStartAt))
	t.Logf("%d clients connected in %s", threads, time.Now().Sub(connStartAt))

	wg.Wait()
	for i := 0; i < threads; i++ {
		if errs[i] != nil {
			t.Fatalf("Test failed: %v", errs[i])
		}
	}
}

func testScalabilitySendAll(t *testing.T, addr string, loops int, threads int) {
	initialErr := errors.New("initial")
	errs := make([]error, threads)

	ssock := multisocket.New(nil)
	if e := ssock.Listen(addr); e != nil {
		t.Fatalf("Cannot listen: %v", e)
	}

	wg := &sync.WaitGroup{}
	wg.Add(threads)
	connStartAt := time.Now()
	for i := 0; i < threads; i++ {
		errs[i] = initialErr
		go scalabilitySendAllClient(t, i, addr, &errs[i], loops, wg)
	}

	// all clients are connected
	var (
		err error
		msg *message.Message
	)
	for i := 0; i < threads; i++ {
		if msg, err = ssock.RecvMsg(); err != nil {
			t.Errorf("recv start signal error: %s", err)
		}
		msg.FreeAll()
		// fmt.Fprintf(os.Stderr, "%d clients connected\n", i)
	}
	// fmt.Fprintf(os.Stderr, "%d clients connected in %s\n", threads, time.Now().Sub(connStartAt))
	t.Logf("%d clients connected in %s", threads, time.Now().Sub(connStartAt))

	sendStartAt := time.Now()
	var (
		content = []byte("hello")
	)
	for i := 0; i < loops; i++ {
		if err = ssock.SendAll(content); err != nil {
			t.Errorf("sendAll error: %s", err)
		}
	}

	wg.Wait()
	t.Logf("send all %d clients %d messages in %s", threads, loops, time.Now().Sub(sendStartAt))
	for i := 0; i < threads; i++ {
		if errs[i] != nil {
			t.Fatalf("Test failed: %v", errs[i])
		}
	}
}

func scalabilitySendAllClient(t *testing.T, idx int, addr string, errp *error, loops int, wg *sync.WaitGroup) {
	defer wg.Done()
	sock := multisocket.New(nil)
	defer sock.Close()
	if err := sock.Dial(addr); err != nil {
		*errp = err
		return
	}
	// fmt.Fprintf(os.Stderr, "CONNECTED: %d\n", idx)
	// t.Logf("CONNECTED: %d", idx)

	var (
		err         error
		msg         *message.Message
		contentRecv = []byte("hello")
	)

	// send start signal
	if err = sock.Send(nil); err != nil {
		t.Errorf("send start signal error: %d, %s", idx, err)
	}
	// fmt.Fprintf(os.Stderr, "%d started\n", idx)

	for i := 0; i < loops; i++ {
		if msg, err = sock.RecvMsg(); err != nil {
			*errp = err
			return
		}
		if !bytes.Equal(msg.Content, contentRecv) {
			*errp = fmt.Errorf("response mismatch: %d/%d", i, idx)
			return
		}
		msg.FreeAll()
		// fmt.Fprintf(os.Stderr, "OK: %d/%d\n", i, idx)
		// t.Logf("OK: %d/%d", i, idx)
	}
	// clean initial error
	*errp = nil
}

func scalabilitySendReplyClient(t *testing.T, idx int, addr string, errp *error, loops int, wg, connWg *sync.WaitGroup) {
	defer wg.Done()
	sock := multisocket.New(nil)
	defer sock.Close()
	if err := sock.Dial(addr); err != nil {
		*errp = err
		return
	}
	// t.Logf("CONNECTED: %d", idx)
	connWg.Done()
	// all clients send recv concurrently
	connWg.Wait()

	var (
		err          error
		content      = []byte("ping")
		msg          *message.Message
		contentReply = []byte("pong")
	)
	for i := 0; i < loops; i++ {
		// Inject a random sleep to avoid pounding the CPU too hard.
		time.Sleep(time.Duration(rand.Int31n(10000)) * time.Microsecond)

		if err = sock.Send(content); err != nil {
			*errp = err
			return
		}

		if msg, err = sock.RecvMsg(); err != nil {
			*errp = err
			return
		}
		if !bytes.Equal(msg.Content, contentReply) {
			*errp = fmt.Errorf("response mismatch: %d/%d", i, idx)
			return
		}
		msg.FreeAll()
		// t.Logf("OK: %d/%d", i, idx)
	}
	// clean initial error
	*errp = nil
}

func scalabilitySendReplyServer(t *testing.T, sock multisocket.Socket) {
	defer sock.Close()
	var (
		err         error
		msg         *message.Message
		repyContent = []byte("pong")
	)
	for {
		if msg, err = sock.RecvMsg(); err != nil {
			return
		}
		if err = sock.SendTo(msg.Source, repyContent); err != nil {
			return
		}
		msg.FreeAll()
	}
}
