package main

import (
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/address"
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/protocol/stream"
	_ "github.com/webee/multisocket/transport/all"
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
	if err := address.Connect(protoStream, streamAddr, options.OptionValues{connector.Options.Pipe.CloseOnEOF: true}); err != nil {
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
				connector.Options.Pipe.RawMode:     true,
				connector.Options.Dialer.DialAsync: true,
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
				connector.Options.Pipe.RawMode:     true,
				connector.Options.Dialer.DialAsync: true,
				connector.Options.Dialer.Reconnect: false,
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
