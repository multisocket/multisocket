package main

import (
	"os"

	"github.com/webee/multisocket/examples"

	"github.com/webee/multisocket/protocol/stream"

	log "github.com/sirupsen/logrus"
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
	// rawAddr := "tcp://127.0.0.1:60001"
	// streamAddr := "tcp://127.0.0.1:60002"
	rawAddr := os.Args[1]
	streamAddr := os.Args[2]

	rawToStream := stream.NewRawToStream()

	if err := rawToStream.RawListen(rawAddr); err != nil {
		log.WithError(err).Panic("raw listen")
	}

	if err := rawToStream.StreamListen(streamAddr); err != nil {
		log.WithError(err).Panic("stream listen")
	}

	examples.StartPProfListen("localhost:6070")

	examples.SetupSignal()
}
