package main

import (
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/webee/multisocket/protocol/stream"

	log "github.com/sirupsen/logrus"
	_ "github.com/webee/multisocket/transport/all"
)

func init() {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})
}

func main() {
	// rawAddr := "tcp://127.0.0.1:60001"
	// streamAddr := "tcp://127.0.0.1:60002"
	rawAddr := os.Args[1]
	streamAddr := os.Args[2]

	rawToStream := stream.NewRawToStream()

	if err := rawToStream.RawListen(rawAddr); err != nil {
		log.WithError(err).Panic("raw listen")
	}

	if err := rawToStream.StreamListen(streamAddr); err != nil {
		log.WithError(err).Panic("stream listen")
	}

	// startPProfListen("localhost:6070")

	setupSignal()
}

// setupSignal register signals handler and waiting for.
func setupSignal() {
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

// startPProfListen start debug pprof listening.
func startPProfListen(addr string) {
	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/debug/pprof/", pprof.Index)
	serverMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	serverMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	serverMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	serverMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	go func() {
		log.Println("pprof listening:", addr)
		if err := http.ListenAndServe(addr, serverMux); err != nil {
			log.Panicln("pprof listening:", err)
		}
	}()
}
