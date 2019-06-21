package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/webee/multisocket/examples"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})
}

func main() {
	if len(os.Args) > 2 && os.Args[1] == "dial" {
		client(os.Args[2])
		os.Exit(0)
	}
	if len(os.Args) > 2 && os.Args[1] == "listen" {
		server(os.Args[2])
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr,
		"Usage: tcp dial|listen <URL> <ARG> ...\n")
	os.Exit(1)
}

func client(addr string) {
	var (
		err     error
		tcpAddr *net.TCPAddr
		conn    net.Conn
	)
	if tcpAddr, err = net.ResolveTCPAddr("tcp", addr); err != nil {
		log.WithField("err", err).Panicf("resolveTCPAddr")
	}

	if conn, err = net.DialTCP("tcp", nil, tcpAddr); err != nil {
		log.WithField("err", err).Panicf("DialTCP")
	}
	handleConn(conn)
}

func server(addr string) {
	var (
		err     error
		tcpAddr *net.TCPAddr
		l       *net.TCPListener
		conn    net.Conn
	)

	if tcpAddr, err = net.ResolveTCPAddr("tcp", addr); err != nil {
		log.WithField("err", err).Panicf("resolveTCPAddr")
	}

	if l, err = net.ListenTCP("tcp", tcpAddr); err != nil {
		log.WithField("err", err).Panicf("ListenTCP")
	}

	for {
		if conn, err = l.Accept(); err != nil {
			log.WithField("err", err).Error("Accept")
			break
		}
		go handleConn(conn)
	}

	examples.SetupSignal()
}

func handleConn(conn net.Conn) {
	go func() {
		// io.Copy(conn, os.Stdin)
		for {
			_, err := conn.Write([]byte("HELLO\n"))
			if err != nil {
				log.WithError(err).Info("[handle done write]")
				break
			}
			time.Sleep(time.Second)
		}
	}()
	_, err := io.Copy(os.Stdout, conn)
	log.WithError(err).Info("[handle done read]")

	// conn.Close()
}
