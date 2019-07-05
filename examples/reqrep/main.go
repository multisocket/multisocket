package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/webee/multisocket/address"

	"github.com/webee/multisocket/examples"

	log "github.com/sirupsen/logrus"
	"github.com/webee/multisocket/protocol/reqrep"
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
	// client("tcp://127.0.0.1:30001#dial", "webee")
	if len(os.Args) > 2 && os.Args[1] == "rep" {
		n, _ := strconv.Atoi(os.Args[3])
		server(os.Args[2], n)
		os.Exit(0)
	}
	if len(os.Args) > 3 && os.Args[1] == "req" {
		client(os.Args[2], os.Args[3])
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr,
		"Usage: reqrep req|rep <URL> <ARG> ...\n")
	os.Exit(1)
}

func server(addr string, n int) {
	rep := reqrep.NewRep(reqHandler(n))
	rep.Start()

	if err := address.Connect(rep, addr); err != nil {
		log.WithField("err", err).Panicf("connect")
	}

	examples.SetupSignal()
}

func client(addr string, name string) {
	req := reqrep.NewReq()

	if err := address.Connect(req, addr); err != nil {
		log.WithField("err", err).Panicf("connect")
	}

	idx := 0
	for {
		log.WithField("id", idx).Infof("request")
		if reply, err := req.Request([]byte(fmt.Sprintf("%s#%d", name, idx))); err != nil {
			log.WithError(err).WithField("id", idx).Errorf("request")
			time.Sleep(1 * time.Second)
		} else {
			fmt.Printf("%s\n", string(reply))
		}
		// time.Sleep(100 * time.Millisecond)
		idx++
	}
}

type reqHandler int

func (h reqHandler) Handle(req []byte) []byte {
	time.Sleep(100 * time.Millisecond)
	return []byte(fmt.Sprintf("[#%d]Hello, %s", int(h), string(req)))
}
