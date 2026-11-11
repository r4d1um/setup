// SPDX-License-Identifier: MIT

package fakebroker

import (
	"bufio"
	"net"
	"sync"
	"testing"
	"time"
)

// a bare-bones MQTT client, enough to drive the broker
type tc struct {
	t    *testing.T
	conn net.Conn
	r    *bufio.Reader
}

func dial(t *testing.T, b *Broker, id, user, pass string, will *message, keepAlive uint16) (*tc, byte) {
	t.Helper()
	conn, err := net.Dial("tcp", b.Addr())
	if err != nil {
		t.Fatal(err)
	}
	flags := byte(0x02)
	body := append(str("MQTT"), 4)
	var payload []byte
	payload = append(payload, str(id)...)
	if will != nil {
		flags |= 0x04 | will.qos<<3
		if will.retain {
			flags |= 0x20
		}
		payload = append(payload, str(will.topic)...)
		payload = append(payload, str(string(will.payload))...)
	}
	if user != "" {
		flags |= 0x80
		payload = append(payload, str(user)...)
	}
	if pass != "" {
		flags |= 0x40
		payload = append(payload, str(pass)...)
	}
	body = append(body, flags)
	body = append(body, u16(keepAlive)...)
	conn.Write(encode(pConnect<<4, append(body, payload...)))
	c := &tc{t: t, conn: conn, r: bufio.NewReader(conn)}
	h, ack := c.read()
	if h>>4 != pConnack || len(ack) != 2 {
		t.Fatalf("no CONNACK: %x %x", h, ack)
	}
	return c, ack[1]
}

func (c *tc) read() (byte, []byte) {
	c.t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	h, b, err := readPacket(c.r)
	if err != nil {
		c.t.Fatalf("read: %v", err)
	}
	return h, b
}

func (c *tc) publish(topic, payload string, qos byte, retain bool) {
	h := byte(pPublish<<4) | qos<<1
	if retain {
		h |= 1
	}
	body := str(topic)
	if qos > 0 {
		body = append(body, u16(7)...)
	}
	c.conn.Write(encode(h, append(body, payload...)))
}

func (c *tc) subscribe(filter string, qos byte) byte {
	c.conn.Write(encode(pSubscribe<<4|2, append(append(u16(1), str(filter)...), qos)))
	h, b := c.read()
	if h>>4 != pSuback {
		c.t.Fatalf("no SUBACK: %x", h)
	}
	return b[2]
}

func (c *tc) expectPublish(topic, payload string, retained bool) {
	c.t.Helper()
	c.expect(topic, payload, &retained)
}

// expect checks the next PUBLISH; retained nil accepts either flag.
func (c *tc) expect(topic, payload string, retained *bool) {
	c.t.Helper()
	h, b := c.read()
	r := &reader{b: b}
	got := string(r.bytes())
	if (h>>1)&3 > 0 {
		r.u16()
	}
	if h>>4 != pPublish || got != topic || string(r.b) != payload || retained != nil && (h&1 != 0) != *retained {
		c.t.Fatalf("got %x %q %q, want %q %q", h, got, r.b, topic, payload)
	}
}

type recorder struct {
	mu sync.Mutex
	ev []Event
}

func (r *recorder) add(e Event) { r.mu.Lock(); r.ev = append(r.ev, e); r.mu.Unlock() }
func (r *recorder) has(kind, topic string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.ev {
		if e.Kind == kind && (topic == "" || e.Topic == topic) {
			return true
		}
	}
	return false
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 300; i++ {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition never became true")
}

func TestPublishSubscribeRetainedQoS(t *testing.T) {
	rec := &recorder{}
	b, err := Listen("127.0.0.1:0", Config{OnEvent: rec.add})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	dev, code := dial(t, b, "parkdisplay", "", "", nil, 30)
	if code != 0 {
		t.Fatalf("connack %d", code)
	}
	dev.publish("parken/status", `{"online":true}`, 0, true)
	dev.publish("parken/display", `{"free":12}`, 1, false)
	if h, _ := dev.read(); h>>4 != pPuback {
		t.Fatalf("no PUBACK: %x", h)
	}
	dev.publish("parken/qos2", "x", 2, false)
	if h, _ := dev.read(); h>>4 != pPubrec {
		t.Fatalf("no PUBREC: %x", h)
	}
	dev.conn.Write(encode(pPubrel<<4|2, u16(7)))
	if h, _ := dev.read(); h>>4 != pPubcomp {
		t.Fatalf("no PUBCOMP: %x", h)
	}

	sub, _ := dial(t, b, "dashboard", "", "", nil, 30)
	if q := sub.subscribe("parken/+", 2); q != 1 {
		t.Fatalf("granted %d, want 1", q)
	}
	sub.expectPublish("parken/status", `{"online":true}`, true) // retained arrives on subscribe
	dev.publish("parken/display", `{"free":11}`, 0, false)
	sub.expectPublish("parken/display", `{"free":11}`, false)

	// ping, then an empty retained message clears the retained one
	sub.conn.Write(encode(pPingreq<<4, nil))
	if h, _ := sub.read(); h>>4 != pPingresp {
		t.Fatal("no PINGRESP")
	}
	dev.publish("parken/status", "", 0, true)
	sub.expectPublish("parken/status", "", false)
	late, _ := dial(t, b, "late", "", "", nil, 30)
	late.subscribe("parken/#", 0)
	dev.publish("parken/display", "last", 0, false)
	late.expectPublish("parken/display", "last", false) // not the cleared status first
	if !rec.has("publish", "parken/qos2") || !rec.has("subscribe", "parken/+") {
		t.Fatalf("events: %+v", rec.ev)
	}
}

func TestLoginRefuseAndWill(t *testing.T) {
	rec := &recorder{}
	b, _ := Listen("127.0.0.1:0", Config{User: "esp", Pass: "geheim", OnEvent: rec.add})
	defer b.Close()
	if _, code := dial(t, b, "x", "esp", "wrong", nil, 30); code != 4 {
		t.Fatalf("bad password: connack %d", code)
	}
	watcher, code := dial(t, b, "w", "esp", "geheim", nil, 30)
	if code != 0 {
		t.Fatalf("good login: connack %d", code)
	}
	watcher.subscribe("status/#", 1)
	dev, _ := dial(t, b, "dev", "esp", "geheim", &message{topic: "status/dev", payload: []byte("offline"), retain: true}, 1)
	dev.conn.Close() // power cut: the will goes out
	// live, or as retained if the broker stored it first: both are fine
	watcher.expect("status/dev", "offline", nil)
	waitFor(t, func() bool { return rec.has("will", "status/dev") })

	// a clean DISCONNECT sends no will
	dev2, _ := dial(t, b, "dev2", "esp", "geheim", &message{topic: "status/dev2", payload: []byte("offline")}, 30)
	dev2.conn.Write(encode(pDisconnect<<4, nil))
	waitFor(t, func() bool { return rec.has("disconnect", "") })
	time.Sleep(50 * time.Millisecond)
	if rec.has("will", "status/dev2") {
		t.Fatal("will sent after a clean disconnect")
	}

	r, _ := Listen("127.0.0.1:0", Config{Refuse: 3})
	defer r.Close()
	if _, code := dial(t, r, "x", "", "", nil, 30); code != 3 {
		t.Fatalf("Refuse: connack %d", code)
	}
}

func TestKeepAliveTimeout(t *testing.T) {
	rec := &recorder{}
	b, _ := Listen("127.0.0.1:0", Config{OnEvent: rec.add})
	defer b.Close()
	dial(t, b, "sleepy", "", "", &message{topic: "s", payload: []byte("gone")}, 1) // 1 s keep alive: lost after 1.5 s
	waitFor(t, func() bool { return rec.has("will", "s") })
}

func TestMatch(t *testing.T) {
	for _, c := range []struct {
		f, t string
		ok   bool
	}{
		{"a/b", "a/b", true}, {"a/+", "a/b", true}, {"a/+", "a/b/c", false}, {"a/#", "a", true},
		{"a/#", "a/b/c", true}, {"#", "x/y", true}, {"+/b", "a/b", true}, {"a/b", "a/c", false},
		{"#", "$SYS/x", false}, {"a", "a/b", false},
	} {
		if Match(c.f, c.t) != c.ok {
			t.Errorf("Match(%q, %q) != %v", c.f, c.t, c.ok)
		}
	}
}
