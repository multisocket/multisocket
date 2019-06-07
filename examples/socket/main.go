package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/webee/multisocket/options"

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
	// client("tcp://127.0.0.1:30001", "webee")
	if len(os.Args) > 2 && os.Args[1] == "server" {
		server(os.Args[2])
		os.Exit(0)
	}
	if len(os.Args) > 3 && os.Args[1] == "client" {
		client(os.Args[2], os.Args[3])
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr,
		"Usage: socket server|client <URL> <ARG> ...\n")
	os.Exit(1)
}

func server(addr string) {
	sock := multisocket.New(connector.New(), sender.New(), receiver.New())

	if err := sock.Listen(addr); err != nil {
		log.WithField("err", err).Panicf("listen")
	}

	worker := func(n int) {
		var (
			err error
			msg *multisocket.Message
		)

		for {
			if msg, err = sock.RecvMsg(); err != nil {
				log.WithField("err", err).Errorf("recv")
				continue
			}
			content := []byte(fmt.Sprintf("[#%d]Hello, %s", n, string(msg.Content)))
			if err = sock.SendTo(msg.Source, content); err != nil {
				log.WithField("err", err).Errorf("send")
			}
		}
	}
	go worker(0)
	go worker(1)

	setupSignal()
}

func client(addr string, name string) {
	var (
		err     error
		content []byte
	)

	sock := multisocket.New(connector.New(),
		sender.NewSendOneWithOptions(
			options.NewOptionValue(sender.OptionSendQueueSize, 11),
		),
		receiver.New())
	if err = sock.Sender().SetOption(sender.OptionTTL, 2); err != nil {
		log.WithField("err", err).Panic("set sender option")
	}

	if err := sock.Dial(addr); err != nil {
		log.WithField("err", err).Panicf("dial")
	}

	go func() {
		t := 0
		for {
			if err = sock.Send([]byte(fmt.Sprintf("%s#%d", name, t))); err != nil {
				log.WithField("err", err).Errorf("send")
			}
			log.WithField("id", t).Infof("send")
			time.Sleep(100 * time.Millisecond)
			t++
		}
	}()

	go func() {
		for {
			if content, err = sock.Recv(); err != nil {
				log.WithField("err", err).Errorf("recv")
			}
			fmt.Printf("%s\n", string(content))
		}
	}()

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
