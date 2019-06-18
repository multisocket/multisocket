package main

import (
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/errs"
	"github.com/webee/multisocket/examples"
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
	time.AfterFunc(time.Second*10, func() {
		// limit connections to 1, keep already connected pipes, can't establish connections more than 1.
		conns.SetOption(connector.OptionConnLimit, 1)
	})

	examples.SetupSignal()
}

type handler int

func (handler) HandlePipeEvent(event connector.PipeEvent, pipe connector.Pipe) {
	switch event {
	case connector.PipeEventAdd:
		pipe.Send([]byte("hello"))
		go readPipe(pipe)
	case connector.PipeEventRemove:
	}
}

func readPipe(pipe connector.Pipe) {
	var (
		err     error
		content []byte
	)
	for {
		if content, err = pipe.Recv(); err != nil {
			log.WithField("err", err).Errorf("recv")
			if err == errs.ErrClosed {
				break
			}
		}
		fmt.Fprintln(os.Stderr, string(content))
	}
}
