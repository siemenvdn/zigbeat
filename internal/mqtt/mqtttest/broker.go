// Package mqtttest plays an MQTT broker for one client, for the tests of the
// packages that speak to one.
package mqtttest

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// Wait is how long a test waits for a packet or a message.
const Wait = 2 * time.Second

// Broker plays an MQTT broker for one client. It answers CONNECT, SUBSCRIBE
// (refusing filters under refused/) and UNSUBSCRIBE, and hands every packet it
// reads to Pkts: the first byte, then the body.
type Broker struct {
	ln   *net.TCPListener // the first connection is served; Dialled reports a second one
	Addr string
	Pkts chan []byte
	Conn chan net.Conn // the client's connection, for a test that cuts it
}

// Dialled reports whether a second client dialled within d: a client that
// reconnects by itself does, zigbeat's do not.
func (b *Broker) Dialled(d time.Duration) bool {
	_ = b.ln.SetDeadline(time.Now().Add(d))
	again, err := b.ln.Accept()
	if err == nil {
		again.Close()
	}
	return err == nil
}

// Start listens on a free port; the broker stops with the test.
func Start(t *testing.T) *Broker {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	b := &Broker{ln: ln.(*net.TCPListener), Addr: ln.Addr().String(), Pkts: make(chan []byte, 64), Conn: make(chan net.Conn, 1)}
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		t.Cleanup(func() { c.Close() })
		b.Conn <- c
		r := bufio.NewReader(c)
		for {
			p, err := readRaw(r)
			if err != nil {
				return
			}
			b.Pkts <- p
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

// Publish sends the client a PUBLISH at QoS 0.
func (b *Broker) Publish(t *testing.T, topic, payload string, retained bool) {
	t.Helper()
	c := <-b.Conn
	b.Conn <- c
	body := append(String(topic), payload...)
	h := byte(0x30)
	if retained {
		h |= 0x01
	}
	if _, err := c.Write(append(binary.AppendUvarint([]byte{h}, uint64(len(body))), body...)); err != nil {
		t.Fatal(err)
	}
}

// Next returns the next packet of the given type that the client sent.
func (b *Broker) Next(t *testing.T, kind byte) []byte {
	t.Helper()
	for {
		select {
		case p := <-b.Pkts:
			if p[0]>>4 == kind {
				return p
			}
		case <-time.After(Wait):
			t.Fatalf("no packet of type %d", kind)
		}
	}
}

// String encodes s as MQTT does: a two-byte length, then the bytes.
func String(s string) []byte {
	return append(binary.BigEndian.AppendUint16(nil, uint16(len(s))), s...)
}
