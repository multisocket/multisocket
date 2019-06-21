package main

import (
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/protocol/stream"
	"github.com/webee/multisocket/transport"
	_ "github.com/webee/multisocket/transport/all"
	"github.com/webee/multisocket/utils/connutils"
)

func init() {
	log.SetLevel(log.InfoLevel)
	log.SetFormatter(&log.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})
}

func main() {
	// examples.StartPProfListen("localhost:6060")

	streamAddr := os.Args[1]
	rawAddr := os.Args[2]

	protoStream := stream.New()
	if err := connutils.ParseSmartAddress(streamAddr).Connect(protoStream, options.OptionValues{connector.PipeOptionCloseOnEOF: true}); err != nil {
		log.WithField("err", err).Panicf("stream connect")
	}

	rawStream := stream.New()

	var (
		err     error
		conn    stream.Connection
		rawConn stream.Connection
	)

	// prepare 3 raw connections
	for i := 0; i < 3; i++ {
		rawStream.DialOptions(rawAddr,
			options.OptionValues{
				transport.OptionConnRawMode:     true,
				connector.DialerOptionDialAsync: true,
			})
	}

	for {
		if conn, err = protoStream.Accept(); err != nil {
			log.WithField("err", err).Error("stream accept")
			break
		}

		// async dial
		rawStream.DialOptions(rawAddr,
			options.OptionValues{
				transport.OptionConnRawMode:     true,
				connector.DialerOptionDialAsync: true,
				connector.DialerOptionReconnect: false,
			})

		if rawConn, err = rawStream.Accept(); err != nil {
			log.WithField("err", err).Error("raw accept")
			break
		}

		go forward(conn, rawConn)
		go forward(rawConn, conn)
	}
}

func forward(fromConn, toConn stream.Connection) {
	// io.Copy(toConn, io.TeeReader(fromConn, os.Stderr))
	io.Copy(toConn, fromConn)
	toConn.Close()
	fromConn.Close()
}
