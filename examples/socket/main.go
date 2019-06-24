// socket example shows the usage of sockets: create, send/recv, close and raw socket

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/webee/multisocket/address"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket"
	"github.com/webee/multisocket/examples"
	"github.com/webee/multisocket/message"
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
	if len(os.Args) > 2 && os.Args[1] == "server" {
		addrs := make([]address.MultiSocketAddress, len(os.Args)-2)
		for i, s := range os.Args[2:] {
			sa, err := address.ParseMultiSocketAddress(s)
			if err != nil {
				log.WithError(err).Panic("ParseMultiSocketAddress")
			}
			addrs[i] = sa
		}
		server(addrs...)
		os.Exit(0)
	}
	if len(os.Args) > 3 && os.Args[1] == "client" {
		addr, err := address.ParseMultiSocketAddress(os.Args[3])
		if err != nil {
			log.WithError(err).Panic("ParseMultiSocketAddress")
		}
		client(os.Args[2], addr)
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "Usage: socket server|client dial|listen <URL> <ARG> ...\n")
	os.Exit(1)
}

func server(addrs ...address.MultiSocketAddress) {
	sock := multisocket.NewDefault()

	for _, addr := range addrs {
		if err := addr.Connect(sock); err != nil {
			log.WithField("err", err).WithFields(log.Fields{"address": addr}).Panicf("connect")
		}
	}

	worker := func(n int) {
		var (
			err error
			msg *message.Message
		)

		for {
			if msg, err = sock.RecvMsg(); err != nil {
				log.WithField("err", err).Errorf("recv")
				continue
			}
			if msg.Content == nil {
				// EOF
				sock.ClosePipe(msg.PipeID())
			} else {
				s := string(msg.Content)
				content := []byte(fmt.Sprintf("[#%d]Hello, %s", n, s))
				if err = sock.SendTo(msg.Source, content); err != nil {
					log.WithField("err", err).Errorf("send")
				}
			}
		}
	}
	// recving concurrently
	go worker(0)
	go worker(1)

	examples.SetupSignal()
}

func client(name string, addr address.MultiSocketAddress) {
	var (
		err     error
		content []byte
	)

	sock := multisocket.NewDefault()

	if err := addr.Connect(sock); err != nil {
		log.WithField("err", err).WithFields(log.Fields{"address": addr}).Panicf("connect")
	}

	// sending
	go func() {
		var content string
		idx := 0
		for {
			content = fmt.Sprintf("%s#%d", name, idx)
			if err = sock.Send([]byte(content)); err != nil {
				log.WithField("err", err).Errorf("send")
			}
			log.WithField("id", idx).Infof("send")
			time.Sleep(1000 * time.Millisecond)
			idx++
		}
	}()

	// recving
	go func() {
		for {
			if content, err = sock.Recv(); err != nil {
				log.WithField("err", err).Errorf("recv")
			}
			fmt.Printf("%s\n", string(content))
		}
	}()

	examples.SetupSignal()
}
