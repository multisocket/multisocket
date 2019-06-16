package main

import (
	"io"
	"os"
	"time"

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
	// x := "server"
	// t := "listen"
	// addr := "tcp://127.0.0.1:30001"
	x := os.Args[1]
	t := os.Args[2]
	addr := os.Args[3]

	protoStream := stream.New()

	switch t {
	case "listen":
		if err := protoStream.Listen(addr); err != nil {
			log.WithField("err", err).Panicf("listen")
		}
	default:
		if err := protoStream.Dial(addr); err != nil {
			log.WithField("err", err).Panicf("dial")
		}
	}

	if x == "server" {
		server(protoStream)
	} else {
		client(protoStream)
	}
}

func server(protoStream stream.Stream) {
	var (
		err  error
		conn io.ReadWriteCloser
	)
	protoStream.SetOption(stream.OptionConnKeepAliveIdle, 10*time.Second)
	protoStream.SetOption(stream.OptionConnKeepAliveInterval, 1*time.Second)

	mr := NewMultiplexReader(os.Stdin)
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
	defer func() {
		conn.Close()
		log.Info("handle done")
	}()
	go io.Copy(conn, src)
	io.Copy(dest, conn)
}
