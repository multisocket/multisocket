package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/webee/multisocket/address"

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
	// client("tcp://127.0.0.1:30001#dial", "webee")
	if len(os.Args) > 3 && os.Args[1] == "push" {
		n, _ := strconv.Atoi(os.Args[3])
		pusher(os.Args[2], n)
		os.Exit(0)
	}
	if len(os.Args) > 2 && os.Args[1] == "pull" {
		puller(os.Args[2])
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr,
		"Usage: pipeline push|pull dial|listen <URL> <ARG> ...\n")
	os.Exit(1)
}

func pusher(addr string, n int) {
	pusher := pipeline.NewPush()

	if err := address.Connect(pusher, addr, options.OptionValues{connector.Options.Dialer.DialAsync: true}); err != nil {
		log.WithField("err", err).Panicf("connect")
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

func puller(addr string) {
	puller := pipeline.NewPull()

	if err := address.Connect(puller, addr, options.OptionValues{connector.Options.Dialer.DialAsync: true}); err != nil {
		log.WithField("err", err).Panicf("connect")
	}

	for {
		content, err := puller.Recv()
		if err != nil {
			log.WithError(err).Error("recv")
			break
		}
		log.Info(string(content))
	}
}
