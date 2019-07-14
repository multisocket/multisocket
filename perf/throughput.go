// Copyright 2019 The Mangos Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package perf provides utilities to measure mangos peformance against
// libnanomsg' perf tools.
// for multisocket

package main

import (
	"fmt"
	"log"
	"time"

	"github.com/webee/multisocket"
	_ "github.com/webee/multisocket/transport/all"
	"github.com/webee/multisocket/transport/tcp"
)

// ThroughputServer is the server side -- very much equivalent to local_thr in
// nanomsg/perf.  It does the measurement by counting packets received.
func ThroughputServer(addr string, msgSize int, count int) {
	s := multisocket.NewDefault()
	defer s.Close()

	l, err := s.NewListener(addr, nil)
	if err != nil {
		log.Fatalf("Failed to make new listener: %v", err)
	}

	// Disable TCP no delay, please! - only valid for TCP
	l.SetOption(tcp.Options.NoDelay, true)

	err = l.Listen()
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	msg, err := s.RecvMsg()
	if err != nil {
		log.Fatalf("Failed to receive start message: %v", err)
	}
	msg.FreeAll()

	start := time.Now()

	for i := 0; i != count; i++ {
		msg, err := s.RecvMsg()
		if err != nil {
			log.Fatalf("Failed to recv: %v", err)
		}
		if len(msg.Content) != msgSize {
			log.Fatalf("Received wrong message size: %d != %d", len(msg.Content), msgSize)
		}
		// return to cache to avoid GC
		msg.FreeAll()
	}

	finish := time.Now()

	delta := finish.Sub(start)
	deltasec := float64(delta) / float64(time.Second)
	msgpersec := float64(count) / deltasec
	mbps := (float64((count)*8*msgSize) / deltasec) / 1000000.0
	fmt.Printf("message size: %d [B]\n", msgSize)
	fmt.Printf("message count: %d\n", count)
	fmt.Printf("throughput: %d [msg/s]\n", uint64(msgpersec))
	fmt.Printf("throughput: %.3f [Mb/s]\n", mbps)
}

// ThroughputClient is the client side of the latency test.  It simply sends
// the requested number of packets of given size to the server.  It corresponds
// to remote_thr.
func ThroughputClient(addr string, msgSize int, count int) {
	s := multisocket.NewDefault()
	defer s.Close()

	d, err := s.NewDialer(addr, nil)
	if err != nil {
		log.Fatalf("Failed to make new dialer: %v", err)
	}

	// Disable TCP no delay, please!
	d.SetOption(tcp.Options.NoDelay, true)

	err = d.Dial()
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}

	// 100 milliseconds to give TCP a chance to establish
	time.Sleep(time.Millisecond * 100)

	content := make([]byte, msgSize)
	for i := 0; i < msgSize; i++ {
		content[i] = 111
	}

	// send the start message
	s.Send(nil)

	for i := 0; i < count; i++ {
		if err = s.Send(content); err != nil {
			log.Fatalf("Failed Send: %v", err)
		}
	}
	log.Println("Client exits")
}
