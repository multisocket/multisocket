package main

import (
	"io"
	"os"

	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/transport"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/protocol/stream"
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
	addr := os.Args[1]
	// addr := "tcp://127.0.0.1:50001"
	protoStream := stream.New()
	if err := protoStream.ListenOptions(addr, options.OptionValues{transport.OptionConnRawMode: true}); err != nil {
		log.WithField("err", err).Panicf("listen")
	}

	var (
		err    error
		stream io.ReadWriteCloser
	)
	for {
		if stream, err = protoStream.Accept(); err != nil {
			log.WithField("err", err).Error("recv stream")
			break
		}
		go handleStream(stream)
	}
}

func handleStream(stream io.ReadWriteCloser) {
	go io.Copy(os.Stdout, stream)
	io.Copy(stream, os.Stdin)

	stream.Close()
}
