package main

import (
	"fmt"
	"os"
	"time"

	"github.com/webee/multisocket/address"

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
	ctr := connector.New()
	ctr.RegisterPipeEventHandler(handler(0))

	sa, err := address.ParseMultiSocketAddress(os.Args[1])
	if err != nil {
		log.WithError(err).Panic("ParseMultiSocketAddress")
	}

	if err := sa.Connect(ctr); err != nil {
		log.WithField("err", err).WithFields(log.Fields{"address": sa}).Panicf("connect")
	}

	time.AfterFunc(time.Second*10, func() {
		// limit connections to 1, keep already connected pipes, can't establish connections more than 1.
		ctr.SetOption(connector.Options.PipeLimit, 1)
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
		time.Sleep(1 * time.Second)
	}
}
