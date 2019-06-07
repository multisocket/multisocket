package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/webee/multisocket"
	"github.com/webee/multisocket/connector"
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
	conns := connector.New()

	conns.RegisterPipeEventHandler(handler(0))

	if err := conns.Listen(os.Args[1]); err != nil {
		log.WithField("err", err).Panicf("listen")
	}
	time.AfterFunc(time.Second*6, func() {
		conns.SetOption(connector.OptionConnLimit, 1)
	})

	setupSignal()
}

type handler int

func (handler) HandlePipeEvent(event multisocket.PipeEvent, pipe socket.Pipe) {
	switch event {
	case multisocket.PipeEventAdd:
		pipe.Send([]byte("hello"))
		go readPipe(pipe)
	case multisocket.PipeEventRemove:
	}
}

func readPipe(pipe socket.Pipe) {
	for {
		if _, err := pipe.Recv(); err != nil {
			log.WithField("err", err).Errorf("recv")
			if err == multisocket.ErrClosed {
				break
			}
		}
	}
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
