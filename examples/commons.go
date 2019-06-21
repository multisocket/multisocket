package examples

import (
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// SetupSignal register signals handler and waiting for.
func SetupSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	for {
		s := <-c
		log.WithField("signal", s.String()).Info("signal")
		switch s {
		case os.Interrupt, syscall.SIGTERM:
			return
		default:
			return
		}
	}
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
