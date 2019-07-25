// +build scaletcp

package test

// This simple test just fires off a crapton of inproc clients, to see
// how many connections we could handle.  We do this using inproc, because
// we would absolutely exhaust TCP ports before we would hit any of the
// natural limits.  The inproc transport has no such limits, so we are
// effectively just testing goroutine scalability, which is what we want.
// The intention is to demonstrate that multisocket can address the C10K problem
// without breaking a sweat.

import (
	"testing"

	_ "github.com/multisocket/multisocket/transport/tcp"
)

func TestScalabilityTCPSendReply(t *testing.T) {
	testScalabilitySendReply(t, "tcp://127.0.0.1:9991", 10, 500)
}

func TestScalabilityTCPSendAll(t *testing.T) {
	testScalabilitySendAll(t, "tcp://127.0.0.1:9992", 10, 1000)
}
