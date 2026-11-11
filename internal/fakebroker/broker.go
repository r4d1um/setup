// SPDX-License-Identifier: MIT

// Package fakebroker is a small MQTT 3.1.1 broker for tests: CONNECT with an
// optional login, PUBLISH QoS 0/1/2, retained messages, SUBSCRIBE with + and #
// (granted QoS at most 1), UNSUBSCRIBE, keep alive, last will, clean disconnect.
// Every event is reported to OnEvent, so a test can check exactly what a device sent.
// Faults on demand: refuse logins (Refuse), drop every connection after N packets (DropAfter).
package fakebroker

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// Event is one thing that happened, in the order it happened.
type Event struct {
	Time    time.Time `json:"time"`
	Kind    string    `json:"event"` // connect, refused, publish, subscribe, unsubscribe, will, disconnect, lost
	Client  string    `json:"client,omitempty"`
	Topic   string    `json:"topic,omitempty"`
	Payload string    `json:"payload,omitempty"`
	QoS     byte      `json:"qos,omitempty"`
	Retain  bool      `json:"retain,omitempty"`
	Detail  string    `json:"detail,omitempty"`
}

type Config struct {
	User, Pass string      // "" accepts any login
	Refuse     byte        // non-zero: answer every CONNECT with this CONNACK code (4 bad login, 5 not authorised, 3 unavailable)
	DropAfter  int         // >0: close each connection after that many packets from the client
	OnEvent    func(Event) // may be nil
	Now        func() time.Time
}

type message struct {
	topic   string
	payload []byte
	qos     byte
	retain  bool
}

type Broker struct {
	cfg      Config
	ln       net.Listener
	mu       sync.Mutex
	retained map[string]message
	clients  map[*client]bool
	wg       sync.WaitGroup
}

type client struct {
	b      *Broker
	conn   net.Conn
	id     string
	subs   map[string]byte // filter -> granted qos
	will   *message
	wmu    sync.Mutex
	nextID uint16
}

// Listen starts a broker on addr ("127.0.0.1:0" picks a free port).
func Listen(addr string, cfg Config) (*Broker, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	b := &Broker{cfg: cfg, ln: ln, retained: map[string]message{}, clients: map[*client]bool{}}
	b.wg.Add(1)
	go b.accept()
	return b, nil
}

func (b *Broker) Addr() string { return b.ln.Addr().String() }

// Close stops listening and drops all clients (without sending their wills).
func (b *Broker) Close() error {
	err := b.ln.Close()
	b.mu.Lock()
	for c := range b.clients {
		c.will = nil
		c.conn.Close()
	}
	b.mu.Unlock()
	b.wg.Wait()
	return err
}

func (b *Broker) event(e Event) {
	if b.cfg.OnEvent != nil {
		e.Time = b.cfg.Now()
		b.cfg.OnEvent(e)
	}
}

func (b *Broker) accept() {
	defer b.wg.Done()
	for {
		conn, err := b.ln.Accept()
		if err != nil {
			return
		}
		c := &client{b: b, conn: conn, subs: map[string]byte{}}
		b.wg.Add(1)
		go func() { defer b.wg.Done(); c.serve() }()
	}
}

// packet types
const (
	pConnect = 1 + iota
	pConnack
	pPublish
	pPuback
	pPubrec
	pPubrel
	pPubcomp
	pSubscribe
	pSuback
	pUnsubscribe
	pUnsuback
	pPingreq
	pPingresp
	pDisconnect
)

func readPacket(r *bufio.Reader) (header byte, body []byte, err error) {
	header, err = r.ReadByte()
	if err != nil {
		return
	}
	var n, mul int = 0, 1
	for i := 0; ; i++ {
		if i == 4 {
			return 0, nil, errors.New("malformed remaining length")
		}
		var d byte
		if d, err = r.ReadByte(); err != nil {
			return
		}
		n += int(d&0x7f) * mul
		mul *= 128
		if d&0x80 == 0 {
			break
		}
	}
	body = make([]byte, n)
	_, err = io.ReadFull(r, body)
	return
}

func encode(header byte, body []byte) []byte {
	out := []byte{header}
	n := len(body)
	for {
		d := byte(n % 128)
		n /= 128
		if n > 0 {
			d |= 0x80
		}
		out = append(out, d)
		if n == 0 {
			break
		}
	}
	return append(out, body...)
}

func (c *client) send(header byte, body []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, err := c.conn.Write(encode(header, body))
	return err
}

type reader struct {
	b   []byte
	err error
}

func (r *reader) u8() byte {
	if r.err != nil || len(r.b) < 1 {
		r.err = errors.New("short packet")
		return 0
	}
	v := r.b[0]
	r.b = r.b[1:]
	return v
}

func (r *reader) u16() uint16 {
	if r.err != nil || len(r.b) < 2 {
		r.err = errors.New("short packet")
		return 0
	}
	v := binary.BigEndian.Uint16(r.b)
	r.b = r.b[2:]
	return v
}

func (r *reader) bytes() []byte {
	n := int(r.u16())
	if r.err != nil || len(r.b) < n {
		r.err = errors.New("short packet")
		return nil
	}
	v := r.b[:n]
	r.b = r.b[n:]
	return v
}

func u16(v uint16) []byte { return []byte{byte(v >> 8), byte(v)} }
func str(s string) []byte { return append(u16(uint16(len(s))), s...) }

func (c *client) serve() {
	defer c.conn.Close()
	r := bufio.NewReader(c.conn)
	keepAlive := 10 * time.Second
	c.conn.SetReadDeadline(time.Now().Add(keepAlive))
	header, body, err := readPacket(r)
	if err != nil || header>>4 != pConnect {
		return
	}
	ka, ok := c.connect(body)
	if !ok {
		return
	}
	if ka > 0 {
		keepAlive = time.Duration(ka) * 1500 * time.Millisecond
	} else {
		keepAlive = 0
	}
	c.b.mu.Lock()
	c.b.clients[c] = true
	c.b.mu.Unlock()
	defer func() {
		c.b.mu.Lock()
		delete(c.b.clients, c)
		c.b.mu.Unlock()
	}()
	for packets := 1; ; packets++ {
		if c.b.cfg.DropAfter > 0 && packets >= c.b.cfg.DropAfter {
			c.lost("dropped by the broker (DropAfter)")
			return
		}
		if keepAlive > 0 {
			c.conn.SetReadDeadline(time.Now().Add(keepAlive))
		} else {
			c.conn.SetReadDeadline(time.Time{})
		}
		header, body, err := readPacket(r)
		if err != nil {
			c.lost(err.Error())
			return
		}
		if done := c.handle(header, body); done {
			return
		}
	}
}

func (c *client) connect(body []byte) (keepAlive uint16, ok bool) {
	r := &reader{b: body}
	proto := string(r.bytes())
	level := r.u8()
	flags := r.u8()
	keepAlive = r.u16()
	c.id = string(r.bytes())
	if flags&0x04 != 0 {
		topic := string(r.bytes())
		payload := append([]byte{}, r.bytes()...)
		c.will = &message{topic: topic, payload: payload, qos: (flags >> 3) & 3, retain: flags&0x20 != 0}
	}
	var user, pass string
	if flags&0x80 != 0 {
		user = string(r.bytes())
	}
	if flags&0x40 != 0 {
		pass = string(r.bytes())
	}
	code := byte(0)
	switch {
	case r.err != nil:
		return 0, false
	case !(proto == "MQTT" && level == 4) && !(proto == "MQIsdp" && level == 3):
		code = 1
	case c.b.cfg.Refuse != 0:
		code = c.b.cfg.Refuse
	case c.b.cfg.User != "" && (user != c.b.cfg.User || pass != c.b.cfg.Pass):
		code = 4
	}
	c.send(pConnack<<4, []byte{0, code})
	if code != 0 {
		c.b.event(Event{Kind: "refused", Client: c.id, Detail: fmt.Sprintf("connack %d, user %q", code, user)})
		return 0, false
	}
	c.b.event(Event{Kind: "connect", Client: c.id, Detail: fmt.Sprintf("user %q, keepalive %ds", user, keepAlive)})
	return keepAlive, true
}

func (c *client) lost(why string) {
	c.b.event(Event{Kind: "lost", Client: c.id, Detail: why})
	if w := c.will; w != nil {
		c.will = nil
		c.b.event(Event{Kind: "will", Client: c.id, Topic: w.topic, Payload: string(w.payload), QoS: w.qos, Retain: w.retain})
		c.b.publish(*w)
	}
}

// handle processes one packet; true ends the connection.
func (c *client) handle(header byte, body []byte) bool {
	r := &reader{b: body}
	switch header >> 4 {
	case pPublish:
		qos := (header >> 1) & 3
		m := message{topic: string(r.bytes()), qos: qos, retain: header&1 != 0}
		var id uint16
		if qos > 0 {
			id = r.u16()
		}
		if r.err != nil || qos == 3 || strings.ContainsAny(m.topic, "+#") {
			c.lost("malformed PUBLISH")
			return true
		}
		m.payload = append([]byte{}, r.b...)
		c.b.event(Event{Kind: "publish", Client: c.id, Topic: m.topic, Payload: string(m.payload), QoS: qos, Retain: m.retain})
		c.b.publish(m)
		switch qos {
		case 1:
			c.send(pPuback<<4, u16(id))
		case 2:
			c.send(pPubrec<<4, u16(id))
		}
	case pPubrel:
		c.send(pPubcomp<<4, u16(r.u16()))
	case pPuback, pPubrec, pPubcomp:
		if header>>4 == pPubrec {
			c.send(pPubrel<<4|2, u16(r.u16()))
		}
	case pSubscribe:
		id := r.u16()
		var granted []byte
		var filters []string
		for len(r.b) > 0 && r.err == nil {
			f := string(r.bytes())
			q := r.u8()
			if q > 1 {
				q = 1
			}
			c.b.mu.Lock()
			c.subs[f] = q
			c.b.mu.Unlock()
			granted = append(granted, q)
			filters = append(filters, f)
		}
		if r.err != nil {
			c.lost("malformed SUBSCRIBE")
			return true
		}
		c.send(pSuback<<4, append(u16(id), granted...))
		c.b.event(Event{Kind: "subscribe", Client: c.id, Topic: strings.Join(filters, ",")})
		c.b.sendRetained(c, filters)
	case pUnsubscribe:
		id := r.u16()
		var filters []string
		for len(r.b) > 0 && r.err == nil {
			f := string(r.bytes())
			c.b.mu.Lock()
			delete(c.subs, f)
			c.b.mu.Unlock()
			filters = append(filters, f)
		}
		c.send(pUnsuback<<4, u16(id))
		c.b.event(Event{Kind: "unsubscribe", Client: c.id, Topic: strings.Join(filters, ",")})
	case pPingreq:
		c.send(pPingresp<<4, nil)
	case pDisconnect:
		c.will = nil
		c.b.event(Event{Kind: "disconnect", Client: c.id})
		return true
	default:
		c.lost(fmt.Sprintf("unexpected packet type %d", header>>4))
		return true
	}
	return false
}

// Match reports whether an MQTT topic filter matches a topic.
func Match(filter, topic string) bool {
	f, t := strings.Split(filter, "/"), strings.Split(topic, "/")
	if strings.HasPrefix(topic, "$") && (f[0] == "#" || f[0] == "+") {
		return false
	}
	for i, part := range f {
		if part == "#" {
			return true
		}
		if i >= len(t) || part != "+" && part != t[i] {
			return false
		}
	}
	return len(f) == len(t)
}

func (b *Broker) publish(m message) {
	b.mu.Lock()
	if m.retain {
		if len(m.payload) == 0 {
			delete(b.retained, m.topic)
		} else {
			b.retained[m.topic] = m
		}
	}
	type target struct {
		c   *client
		qos byte
	}
	var targets []target
	for c := range b.clients {
		best, hit := byte(0), false
		for f, q := range c.subs {
			if Match(f, m.topic) {
				hit = true
				if q > best {
					best = q
				}
			}
		}
		if hit {
			targets = append(targets, target{c, min(best, m.qos)})
		}
	}
	b.mu.Unlock()
	for _, t := range targets {
		t.c.deliver(m, t.qos, false)
	}
}

func (b *Broker) sendRetained(c *client, filters []string) {
	b.mu.Lock()
	var out []message
	for _, m := range b.retained {
		for _, f := range filters {
			if Match(f, m.topic) {
				out = append(out, m)
				break
			}
		}
	}
	b.mu.Unlock()
	for _, m := range out {
		c.deliver(m, min(m.qos, 1), true)
	}
}

func (c *client) deliver(m message, qos byte, retain bool) {
	header := byte(pPublish << 4)
	if retain {
		header |= 1
	}
	body := str(m.topic)
	if qos > 0 {
		header |= qos << 1
		c.wmu.Lock()
		c.nextID++
		if c.nextID == 0 {
			c.nextID = 1
		}
		id := c.nextID
		c.wmu.Unlock()
		body = append(body, u16(id)...)
	}
	c.send(header, append(body, m.payload...))
}
