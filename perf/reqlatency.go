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

	"github.com/multisocket/multisocket"
	"github.com/multisocket/multisocket/message"
	_ "github.com/multisocket/multisocket/transport/all"
	"github.com/multisocket/multisocket/transport/tcp"
)

// ReqRepLatencyServer is the server side for REQ/REP latency testing.
func ReqRepLatencyServer(addr string, msgSize int, roundTrips int) {
	s := multisocket.NewDefault()
	defer func() { time.Sleep(10 * time.Microsecond); s.Close() }()

	l, err := s.NewListener(addr, nil)
	if err != nil {
		log.Fatalf("Failed to make new listener: %v", err)
	}

	// TCP no delay, please!
	l.SetOption(tcp.Options.NoDelay, true)

	err = l.Listen()
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	for i := 0; i != roundTrips; i++ {
		msg, err := s.RecvMsg()
		if err != nil {
			log.Fatalf("Failed to recv: %v", err)
		}
		if len(msg.Content) != msgSize {
			log.Fatalf("Received wrong message size: %d != %d", len(msg.Content), msgSize)
		}
		if err = s.SendTo(msg.Source, msg.Content); err != nil {
			log.Fatalf("Failed to send: %v", err)
		}
		msg.FreeAll()
	}
}

// ReqRepLatencyClient is the client side of the latency test.  It measures
// round trip times using REQ/REP protocol.
func ReqRepLatencyClient(addr string, msgSize int, roundTrips int) {
	s := multisocket.NewDefault()
	defer s.Close()

	d, err := s.NewDialer(addr, nil)
	if err != nil {
		log.Fatalf("Failed to make new dialer: %v", err)
	}

	// TCP no delay, please!
	d.SetOption(tcp.Options.NoDelay, true)

	err = d.Dial()
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}

	// 100 milliseconds to give TCP a chance to establish
	time.Sleep(time.Millisecond * 100)

	var (
		msg     *message.Message
		content = make([]byte, msgSize)
	)

	total := time.Duration(0)
	for i := 0; i < roundTrips; i++ {
		start := time.Now()
		if err = s.Send(content); err != nil {
			log.Fatalf("Failed Send: %v", err)
		}
		if msg, err = s.RecvMsg(); err != nil {
			log.Fatalf("Failed RecvMsg: %v", err)
		}
		total += time.Since(start)
		msg.FreeAll()
	}

	lat := float64(total/time.Microsecond) / float64(roundTrips)
	fmt.Printf("message size: %d [B]\n", msgSize)
	fmt.Printf("round trip count: %d\n", roundTrips)
	fmt.Printf("average RTT: %.3f [us]\n", lat)
}
