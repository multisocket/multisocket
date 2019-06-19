package main

import (
	"os"
	"time"

	"github.com/webee/multisocket/examples"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/protocol/stream"
	_ "github.com/webee/multisocket/transport/all"
	"github.com/webee/multisocket/utils/connutils"
)

func init() {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})
}

func main() {
	backAddr := os.Args[1]
	frontAddr := os.Args[2]

	backStream, frontStream := stream.NewProxy()
	if err := connutils.ParseSmartAddress(backAddr).Connect(backStream, nil); err != nil {
		log.WithField("err", err).WithField("stream", "back").Panicf("connect")
	}
	backStream.SetOption(stream.OptionConnKeepAliveIdle, 10*time.Second)

	if err := connutils.ParseSmartAddress(frontAddr).Connect(frontStream, nil); err != nil {
		log.WithField("err", err).WithField("stream", "front").Panicf("connect")
	}
	frontStream.SetOption(stream.OptionConnKeepAliveIdle, 10*time.Second)

	examples.SetupSignal()
}
