// socket example shows the usage of sockets: create, send/recv, close and raw socket

package main

import (
	"fmt"
	"net"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket"
	"github.com/webee/multisocket/options"
	_ "github.com/webee/multisocket/transport/all"
	"github.com/webee/multisocket/transport/ws"
)

func init() {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})
}

func main() {
	addr := "ws://127.0.0.1:5555/ws"
	sock := multisocket.NewDefault()
	listener, err := sock.NewListener(addr, options.OptionValues{ws.Options.Listener.ExternalListen: true})
	if err != nil {
		log.WithError(err).WithFields(log.Fields{"address": addr}).Panicf("listen")
	}
	if err := listener.Listen(); err != nil {
		log.WithError(err).WithFields(log.Fields{"address": addr}).Panicf("listen")
	}

	wsListener := listener.TransportListener().(*ws.Listener)
	// var taddr *net.TCPAddr
	// if taddr, err = transport.ResolveTCPAddr("127.0.0.1:5555"); err != nil {
	// 	log.WithError(err).WithFields(log.Fields{"address": addr}).Panicf("resolve tcp addr")
	// }

	tcpListener, _ := net.Listen("tcp", wsListener.URL.Host)
	// if listener, err = net.ListenTCP("tcp", taddr); err != nil {
	// 	return
	// }
	htsvr := &http.Server{Handler: wsListener.ServeMux}
	go htsvr.Serve(tcpListener)

	for {
		msg, err := sock.RecvMsg()
		if err != nil {
			log.WithField("err", err).Errorf("recv")
			continue
		}
		if msg.Content == nil {
			// EOF
			sock.ClosePipe(msg.PipeID())
		} else {
			s := string(msg.Content)
			content := []byte(fmt.Sprintf("Hello, %s", s))
			log.Debugf("to send: %s", string(content))
			if err = sock.SendTo(msg.Source, content); err != nil {
				log.WithField("err", err).Errorf("send")
			}
		}
		msg.FreeAll()
	}
}
