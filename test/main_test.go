package test

import (
	"os"
	"testing"
	// "github.com/webee/multisocket/examples"
)

func TestMain(m *testing.M) {
	// examples.StartPProfListen(":6060")
	// call flag.Parse() here if TestMain uses flags
	os.Exit(m.Run())
}
