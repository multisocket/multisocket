package test

import (
	"net/http"
	"os"
	"testing"

	// pprof
	_ "net/http/pprof"

	log "github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	// StartPProfListen(":6060")
	// call flag.Parse() here if TestMain uses flags
	os.Exit(m.Run())
}

// StartPProfListen start debug pprof listening.
func StartPProfListen(addr string) {
	go func() {
		log.Println("pprof listening:", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Panicln("pprof listening:", err)
		}
	}()
}
