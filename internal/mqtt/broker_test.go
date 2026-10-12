package mqtt

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"strings"
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
