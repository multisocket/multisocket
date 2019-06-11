package main

import (
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"

	_ "github.com/webee/multisocket/transport/all"

	"github.com/webee/multisocket"
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/receiver"
	"github.com/webee/multisocket/sender"
)

func init() {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})
}

func main() {
	tIn := os.Args[1]
	addrIn := os.Args[2]
	tOut := os.Args[3]
	addrOut := os.Args[4]

	sockIn := multisocket.New(connector.New(), sender.New(), receiver.New())
	switch tIn {
	case "dial":
		if err := sockIn.Dial(addrIn); err != nil {
			log.WithField("err", err).Panicf("dial")
		}
	default:
		if err := sockIn.Listen(addrIn); err != nil {
			log.WithField("err", err).Panicf("listen")
		}
	}

	sockOut := multisocket.New(connector.New(), sender.New(), receiver.New())
	switch tOut {
	case "dial":
		if err := sockOut.Dial(addrOut); err != nil {
			log.WithField("err", err).Panicf("dial")
		}
	default:
		if err := sockOut.Listen(addrOut); err != nil {
			log.WithField("err", err).Panicf("listen")
		}
	}

	go forward(sockIn, sockOut)
	go forward(sockOut, sockIn)
	setupSignal()
}

func forward(from multisocket.Socket, to multisocket.Socket) {
	var (
		err error
		msg *multisocket.Message
	)
	for {
		if msg, err = from.RecvMsg(); err != nil {
			log.WithField("err", err).Errorf("recv")
			return
		}
		if err = to.SendMsg(msg); err != nil {
			log.WithField("err", err).Errorf("forward")
			return
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
