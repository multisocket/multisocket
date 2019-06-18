// socket example shows the usage of sockets: create, send/recv, close and raw socket

package main

import (
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket"
	"github.com/webee/multisocket/examples"
	"github.com/webee/multisocket/message"
	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/transport"
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
	// server("listen", "tcp://127.0.0.1:30001", "tcp://127.0.0.1:30002")
	// client("tcp://127.0.0.1:30001", "webee")
	if len(os.Args) > 3 && os.Args[1] == "server" {
		rawAddr := ""
		if len(os.Args) > 4 {
			rawAddr = os.Args[4]
		}
		server(os.Args[2], os.Args[3], rawAddr)
		os.Exit(0)
	}
	if len(os.Args) > 4 && os.Args[1] == "client" {
		client(os.Args[2], os.Args[3], os.Args[4])
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr,
		"Usage: socket server|client dial|listen <URL> <ARG> ...\n")
	os.Exit(1)
}

func server(t, addr, rawAddr string) {
	sock := multisocket.NewDefault()

	switch t {
	case "dial":
		if err := sock.Dial(addr); err != nil {
			log.WithField("err", err).Panicf("dial")
		}
		if rawAddr != "" {
			if err := sock.DialOptions(rawAddr, options.OptionValues{transport.OptionConnRawMode: true}); err != nil {
				log.WithField("err", err).Panicf("dial raw")
			}
		}
	default:
		if err := sock.Listen(addr); err != nil {
			log.WithField("err", err).Panicf("listen")
		}
		if rawAddr != "" {
			if err := sock.ListenOptions(rawAddr, options.OptionValues{transport.OptionConnRawMode: true}); err != nil {
				log.WithField("err", err).Panicf("listen raw")
			}
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

			s := string(msg.Content)
			content := []byte(fmt.Sprintf("[#%d]Hello, %s", n, s))
			if err = sock.SendTo(msg.Source, content); err != nil {
				log.WithField("err", err).Errorf("send")
			}
		}
	}
	// recving concurrently
	go worker(0)
	go worker(1)

	examples.SetupSignal()
}

func client(t, addr string, name string) {
	var (
		err     error
		content []byte
	)

	sock := multisocket.NewDefault()

	switch t {
	case "listen":
		if err := sock.Listen(addr); err != nil {
			log.WithField("err", err).Panicf("listen")
		}
	default:
		if err := sock.Dial(addr); err != nil {
			log.WithField("err", err).Panicf("dial")
		}
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
