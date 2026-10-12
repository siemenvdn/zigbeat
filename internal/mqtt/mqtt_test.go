package mqtt

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

const testWait = 2 * time.Second

// broker plays an MQTT broker for one client. It answers CONNECT, SUBSCRIBE
// (refusing filters under refused/) and UNSUBSCRIBE, and hands every packet it
// reads to pkts: the first byte, then the body.
type broker struct {
	addr string
	pkts chan []byte
	conn chan net.Conn
}

func startBroker(t *testing.T) *broker {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	b := &broker{addr: ln.Addr().String(), pkts: make(chan []byte, 64), conn: make(chan net.Conn, 1)}
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		t.Cleanup(func() { c.Close() })
		b.conn <- c
		r := bufio.NewReader(c)
		for {
			p, err := readRaw(r)
			if err != nil {
				return
			}
			b.pkts <- p
			switch p[0] >> 4 {
			case 1: // CONNECT
				_, _ = c.Write([]byte{0x20, 2, 0, 0})
			case 8: // SUBSCRIBE
				code := byte(0)
				if strings.HasPrefix(string(p[5:]), "refused/") {
					code = 0x80
				}
				_, _ = c.Write([]byte{0x90, 3, p[1], p[2], code})
			case 10: // UNSUBSCRIBE
				_, _ = c.Write([]byte{0xb0, 2, p[1], p[2]})
			}
		}
	}()
	return b
}

// readRaw reads one packet: its first byte and its body.
func readRaw(r *bufio.Reader) ([]byte, error) {
	h, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	n, err := binary.ReadUvarint(r) // MQTT's remaining length is a uvarint
	if err != nil {
		return nil, err
	}
	p := make([]byte, 1+n)
	p[0] = h
	_, err = io.ReadFull(r, p[1:])
	return p, err
}

// publish sends the client a PUBLISH at QoS 0.
func (b *broker) publish(t *testing.T, topic, payload string, retained bool) {
	t.Helper()
	c := <-b.conn
	b.conn <- c
	body := append(mqttString(topic), payload...)
	h := byte(0x30)
	if retained {
		h |= 0x01
	}
	if _, err := c.Write(append(binary.AppendUvarint([]byte{h}, uint64(len(body))), body...)); err != nil {
		t.Fatal(err)
	}
}

// next returns the next packet of the given type that the client sent.
func (b *broker) next(t *testing.T, kind byte) []byte {
	t.Helper()
	for {
		select {
		case p := <-b.pkts:
			if p[0]>>4 == kind {
				return p
			}
		case <-time.After(testWait):
			t.Fatalf("no packet of type %d", kind)
		}
	}
}

func mqttString(s string) []byte {
	return append(binary.BigEndian.AppendUint16(nil, uint16(len(s))), s...)
}

func dial(t *testing.T, b *broker, o Options) *Client {
	t.Helper()
	c, err := Dial(b.addr, o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// inbox collects the payloads a handler gets.
type inbox chan string

func (in inbox) handle(_ string, p []byte, _ time.Time) { in <- string(p) }

func (in inbox) want(t *testing.T, payload string) {
	t.Helper()
	select {
	case got := <-in:
		if got != payload {
			t.Fatalf("got %q, want %q", got, payload)
		}
	case <-time.After(testWait):
		t.Fatalf("no message, want %q", payload)
	}
}

func TestWillAndRetain(t *testing.T) {
	b := startBroker(t)
	c := dial(t, b, Options{ClientID: "id", Username: "u", Password: "p",
		WillTopic: "zigbeat/availability", WillPayload: []byte("offline")})
	want := append(mqttString("MQTT"), 4, 0x02|0x04|0x20|0x40|0x80, 0, byte(defaultKeepAlive/time.Second))
	for _, s := range []string{"id", "zigbeat/availability", "offline", "u", "p"} {
		want = append(want, mqttString(s)...)
	}
	if p := b.next(t, 1); !bytes.Equal(p[1:], want) {
		t.Errorf("CONNECT % x, want % x", p[1:], want)
	}
	if err := c.PublishRetained("zigbeat/state", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if p := b.next(t, 3); p[0] != 0x31 || !bytes.Equal(p[1:], append(mqttString("zigbeat/state"), "{}"...)) {
		t.Errorf("PUBLISH % x", p)
	}
}

func TestSubscribeLiveSkipsRetained(t *testing.T) {
	b := startBroker(t)
	c := dial(t, b, Options{ClientID: "id"})
	in := make(inbox, 2)
	if err := c.SubscribeLive("a/b", in.handle); err != nil {
		t.Fatal(err)
	}
	b.publish(t, "a/b", "retained", true)
	b.publish(t, "a/b", "live", false)
	in.want(t, "live")
}

// TestRetainedThroughAWildcard: serve gets the kept style choices this way.
func TestRetainedThroughAWildcard(t *testing.T) {
	b := startBroker(t)
	c := dial(t, b, Options{ClientID: "id"})
	in := make(inbox, 1)
	if err := c.Subscribe("zigbeat/style/+/set", in.handle); err != nil {
		t.Fatal(err)
	}
	b.publish(t, "zigbeat/style/brightness/set", "30", true)
	in.want(t, "30")
}

// TestHandlersRunInOrderOneAtATime: serve's newest() relies on one sender.
func TestHandlersRunInOrderOneAtATime(t *testing.T) {
	b := startBroker(t)
	c := dial(t, b, Options{ClientID: "id"})
	const n = 40
	var mu sync.Mutex
	var got []string
	busy, overlap := false, false
	done := make(chan struct{})
	handle := func(_ string, p []byte, _ time.Time) {
		mu.Lock()
		overlap = overlap || busy
		busy = true
		mu.Unlock()
		time.Sleep(100 * time.Microsecond)
		mu.Lock()
		busy = false
		if got = append(got, string(p)); len(got) == n {
			close(done)
		}
		mu.Unlock()
	}
	for _, f := range []string{"a/+", "b/+"} {
		if err := c.Subscribe(f, handle); err != nil {
			t.Fatal(err)
		}
	}
	var want []string
	for i := range n {
		want = append(want, string(rune('0'+i)))
		b.publish(t, []string{"a/x", "b/x"}[i%2], want[i], false)
	}
	select {
	case <-done:
	case <-time.After(testWait):
		t.Fatal("not every message arrived")
	}
	mu.Lock()
	defer mu.Unlock()
	if overlap || strings.Join(got, "") != strings.Join(want, "") {
		t.Errorf("overlap %v, order %q, want %q", overlap, strings.Join(got, ""), strings.Join(want, ""))
	}
}

func TestRefusedSubscribeLeavesNothing(t *testing.T) {
	b := startBroker(t)
	c := dial(t, b, Options{ClientID: "id"})
	refused, ok := make(inbox, 1), make(inbox, 1)
	if err := c.Subscribe("refused/+", refused.handle); err == nil {
		t.Fatal("a refused subscription was accepted")
	}
	if err := c.Subscribe("ok", ok.handle); err != nil {
		t.Fatal(err)
	}
	b.publish(t, "refused/a", "stray", false)
	b.publish(t, "ok", "sync", false)
	ok.want(t, "sync")
	if len(refused) != 0 {
		t.Error("the handler of a refused subscription got a message")
	}
}

func TestUnsubscribe(t *testing.T) {
	b := startBroker(t)
	c := dial(t, b, Options{ClientID: "id"})
	gone, ok := make(inbox, 1), make(inbox, 1)
	if err := c.Subscribe("a/b", gone.handle); err != nil {
		t.Fatal(err)
	}
	if err := c.Subscribe("ok", ok.handle); err != nil {
		t.Fatal(err)
	}
	if err := c.Unsubscribe("a/b"); err != nil {
		t.Fatal(err)
	}
	if p := b.next(t, 10); !bytes.HasSuffix(p, mqttString("a/b")) {
		t.Errorf("UNSUBSCRIBE % x", p)
	}
	b.publish(t, "a/b", "stray", false)
	b.publish(t, "ok", "sync", false)
	ok.want(t, "sync")
	if len(gone) != 0 {
		t.Error("an unsubscribed handler got a message")
	}
}

func TestDoneOnCloseAndOnADrop(t *testing.T) {
	b := startBroker(t)
	c := dial(t, b, Options{ClientID: "id"})
	c.Close()
	select {
	case <-c.Done():
	case <-time.After(testWait):
		t.Fatal("Done after a local close")
	}
	if !c.Closed() || c.Err() != nil {
		t.Errorf("closed %v, err %v after a local close", c.Closed(), c.Err())
	}

	b = startBroker(t)
	c = dial(t, b, Options{ClientID: "id"})
	conn := <-b.conn
	conn.Close()
	select {
	case <-c.Done():
	case <-time.After(testWait):
		t.Fatal("Done after the broker dropped the connection")
	}
	if c.Err() == nil {
		t.Error("no error after the broker dropped the connection")
	}
}
