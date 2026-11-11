// SPDX-License-Identifier: MIT

// fake-mqtt-broker is an MQTT 3.1.1 broker for tests that logs every event as one
// JSON line (UTC times), so a test can check what a device or service sent.
//
//	fake-mqtt-broker -listen 127.0.0.1:0 -addr-file /tmp/broker.addr -log events.jsonl
//	fake-mqtt-broker -user esp -pass geheim        # logins other than this get CONNACK 4
//	fake-mqtt-broker -refuse 3                     # every CONNECT: "server unavailable"
//	fake-mqtt-broker -drop-after 5                 # cut each connection after 5 packets (tests reconnects)
//
// -addr-file gets the real address (useful with port 0), written under a temporary name
// and renamed, so a test can wait for it to appear.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/r4d1um/setup/internal/fakebroker"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:1883", "address to listen on (port 0: any free port)")
	user := flag.String("user", "", "required user name (empty: any login)")
	pass := flag.String("pass", "", "required password")
	refuse := flag.Int("refuse", 0, "answer every CONNECT with this CONNACK code")
	dropAfter := flag.Int("drop-after", 0, "close each connection after this many packets")
	logFile := flag.String("log", "", "write events here as JSON lines (default stdout)")
	addrFile := flag.String("addr-file", "", "write the listening address to this file")
	flag.Parse()

	out := os.Stdout
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fail(err)
		}
		defer f.Close()
		out = f
	}
	var mu sync.Mutex
	enc := json.NewEncoder(out)
	b, err := fakebroker.Listen(*listen, fakebroker.Config{
		User: *user, Pass: *pass, Refuse: byte(*refuse), DropAfter: *dropAfter,
		OnEvent: func(e fakebroker.Event) { mu.Lock(); enc.Encode(e); mu.Unlock() },
	})
	if err != nil {
		fail(err)
	}
	if *addrFile != "" {
		tmp := *addrFile + ".tmp"
		if err := os.WriteFile(tmp, []byte(b.Addr()+"\n"), 0o644); err != nil {
			fail(err)
		}
		if err := os.Rename(tmp, *addrFile); err != nil {
			fail(err)
		}
	}
	fmt.Fprintln(os.Stderr, "fake-mqtt-broker: listening on", b.Addr())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	b.Close()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "fake-mqtt-broker:", err)
	os.Exit(1)
}
