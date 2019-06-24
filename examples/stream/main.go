package main

import (
	"io"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/address"
	"github.com/webee/multisocket/examples"
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
	x := os.Args[1]
	addrs := os.Args[2:]
	protoStream := stream.New()
	for _, addr := range addrs {
		if err := address.Connect(protoStream, addr); err != nil {
			log.WithField("err", err).Panicf("connect")
		}
	}

	protoStream.SetOption(stream.Options.ConnKeepAliveIdle, 10*time.Second)

	if x == "server" {
		server(protoStream)
	} else {
		client(protoStream)
	}
}

func server(protoStream stream.Stream) {
	var (
		err  error
		conn stream.Connection
	)
	mr := examples.NewMultiplexReader(os.Stdin)
	for {
		if conn, err = protoStream.Accept(); err != nil {
			log.WithField("err", err).Error("accept")
			break
		}
		xb, _ := mr.NewReader()
		go func() {
			handleConn(conn, xb, os.Stdout)
			xb.Close()
		}()
	}
}

func client(protoStream stream.Stream) {
	var (
		err  error
		conn io.ReadWriteCloser
	)
	if conn, err = protoStream.Connect(0); err != nil {
		log.WithField("err", err).Panic("connect")
	}

	handleConn(conn, os.Stdin, os.Stdout)
}

func handleConn(conn io.ReadWriteCloser, src io.Reader, dest io.Writer) {
	go func() {
		io.Copy(conn, src)
		conn.Close()
	}()

	io.Copy(dest, conn)
	conn.Close()
}
