package main

import (
	"github.com/webee/multisocket/examples"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket"
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
	tFront := os.Args[1]
	addrFront := os.Args[2]
	tBack := os.Args[3]
	addBack := os.Args[4]

	sockFront := multisocket.NewDefault()
	switch tFront {
	case "dial":
		if err := sockFront.Dial(addrFront); err != nil {
			log.WithField("err", err).Panicf("dial")
		}
	default:
		if err := sockFront.Listen(addrFront); err != nil {
			log.WithField("err", err).Panicf("listen")
		}
	}

	sockBack := multisocket.NewDefault()
	switch tBack {
	case "dial":
		if err := sockBack.Dial(addBack); err != nil {
			log.WithField("err", err).Panicf("dial")
		}
	default:
		if err := sockBack.Listen(addBack); err != nil {
			log.WithField("err", err).Panicf("listen")
		}
	}

	go forward(sockFront, sockBack)
	go forward(sockBack, sockFront)
	examples.SetupSignal()
}

func forward(from multisocket.Socket, to multisocket.Socket) {
	for {
		msg, err := from.RecvMsg()
		if err != nil {
			log.WithField("err", err).Errorf("recv")
		}

		if err := to.SendMsg(msg); err != nil {
			log.WithField("err", err).Errorf("forward")
		}
	}
}
