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
	if err := protoStream.ListenOptions(addr, options.NewOptions().WithOption(transport.OptionConnRawMode, true)); err != nil {
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
	defer stream.Close()
	// r := bufio.NewReader(stream)
	// s, _ := r.ReadString('\n')
	// log.Info(s)
	// stream.Write([]byte("hello\n"))

	// time.Sleep(1 * time.Second)
	go io.Copy(os.Stdout, stream)
	// var n int
	// p := make([]byte, 4096)
	// for {
	// 	n, _ = os.Stdin.Read(p)
	// 	msg := p[:n]
	// 	fmt.Fprintf(os.Stderr, "[s]>>>>>>>>>>>>>>\n")
	// 	fmt.Fprintf(os.Stderr, "%s", msg)
	// 	fmt.Fprintf(os.Stderr, "\n[e]>>>>>>>>>>>>>>\n")
	// }
	io.Copy(stream, os.Stdin)

	// b := make([]byte, 1024)
	// for {
	// 	n, _ := os.Stdin.Read(b)
	// 	if n > 0 {
	// 		stream.Write(b[:n])
	// 	}
	// }
}
