package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/options"

	"github.com/webee/multisocket/protocol/pipeline"

	log "github.com/sirupsen/logrus"
	_ "github.com/webee/multisocket/transport/all"
)

func init() {
	log.SetLevel(log.TraceLevel)
	log.SetFormatter(&log.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})
}

func main() {
	// client("dial", "tcp://127.0.0.1:30001", "webee")
	if len(os.Args) > 4 && os.Args[1] == "push" {
		n, _ := strconv.Atoi(os.Args[4])
		pusher(os.Args[2], os.Args[3], n)
		os.Exit(0)
	}
	if len(os.Args) > 3 && os.Args[1] == "pull" {
		puller(os.Args[2], os.Args[3])
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr,
		"Usage: pipeline push|pull dial|listen <URL> <ARG> ...\n")
	os.Exit(1)
}

func pusher(t, addr string, n int) {
	pusher := pipeline.NewPush()
	switch t {
	case "dial":
		if err := pusher.DialOptions(addr, options.OptionValues{connector.DialerOptionDialAsync: true}); err != nil {
			log.WithField("err", err).Panicf("dial")
		}
	default:
		if err := pusher.Listen(addr); err != nil {
			log.WithField("err", err).Panicf("listen")
		}
	}

	idx := 0
	for {
		content := []byte(fmt.Sprintf("#%d: %d", n, idx))
		if err := pusher.Send(content); err != nil {
			log.WithError(err).Error("send")
			break
		}

		time.Sleep(100 * time.Millisecond)
		idx++
	}
}

func puller(t, addr string) {
	puller := pipeline.NewPull()
	switch t {
	case "listen":
		if err := puller.Listen(addr); err != nil {
			log.WithField("err", err).Panicf("listen")
		}
	default:
		if err := puller.DialOptions(addr, options.OptionValues{connector.DialerOptionDialAsync: true}); err != nil {
			log.WithField("err", err).Panicf("dial")
		}
	}

	for {
		content, err := puller.Recv()
		if err != nil {
			log.WithError(err).Error("recv")
			break
		}
		fmt.Println(string(content))
	}
}
