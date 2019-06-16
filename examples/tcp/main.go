package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

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
	setupSignal()
}

func handleConn(conn net.Conn) {
	go io.Copy(conn, os.Stdin)
	io.Copy(os.Stdout, conn)

	// var (
	// 	n   int
	// 	err error
	// )
	// p := make([]byte, 32*1024)
	// for {
	// 	n, err = conn.Read(p)
	// 	if n > 0 {
	// 		msg := p[0:n]
	// 		fmt.Fprintf(os.Stdout, "[%d]%s", len(msg), msg)
	// 		if err != nil {
	// 			break
	// 		}
	// 	}
	// 	if err != nil {
	// 		break
	// 	}
	// }
}

// setupSignal register signals handler and waiting for.
func setupSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	for {
		s := <-c
		log.WithField("signal", s.String()).Info("signal")
		switch s {
		case os.Interrupt, syscall.SIGTERM:
			return
		default:
			return
		}
	}
}
