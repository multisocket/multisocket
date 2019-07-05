package main

import (
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	_ "github.com/webee/multisocket/transport/all"

	"github.com/webee/multisocket"
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/examples"
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
	scheme := "inproc"
	if len(os.Args) > 1 {
		scheme += "." + os.Args[1]
	}

	produceSock := multisocket.New(connector.New(), sender.New(), nil)
	consumeSock := multisocket.New(connector.New(), nil, receiver.New())

	go produce(produceSock)
	go consume(0, consumeSock)
	go consume(1, consumeSock)

	addr := scheme + "://producer"
	if err := produceSock.Listen(addr); err != nil {
		log.WithError(err).Panic("listen")
	}
	if err := consumeSock.Dial(addr); err != nil {
		log.WithError(err).Panic("dial")
	}

	examples.SetupSignal()
}

func produce(sock multisocket.Socket) {
	idx := 0
	for {
		content := []byte(fmt.Sprintf("msg#%d", idx))
		if err := sock.Send(content); err != nil {
			log.WithError(err).Error("send")
			break
		}
		time.Sleep(100 * time.Millisecond)
		idx++
	}
}

func consume(id int, sock multisocket.Socket) {
	for {
		content, err := sock.Recv()
		if err != nil {
			log.WithError(err).Error("recv")
			break
		}
		log.WithField("content", string(content)).
			WithField("consumerID", id).
			Info("consume")
	}
}
