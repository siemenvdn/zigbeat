package mqtt

import (
	"bufio"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// pipeClient is a Client on one end of a net.Pipe; the test plays the broker on the other.
func pipeClient(t *testing.T) (*Client, net.Conn) {
	t.Helper()
	conn, broker := net.Pipe()
	c := &Client{conn: conn, r: bufio.NewReader(conn), ka: defaultKeepAlive, acks: map[uint16]chan []byte{}, done: make(chan struct{})}
	go c.readLoop()
	t.Cleanup(func() { c.shutdown(nil); broker.Close() })
	return c, broker
}

// answer reads one packet as the broker and, when reply is set, answers it with
// the packet's identifier and code.
func answer(broker net.Conn, reply byte, code byte) chan []byte {
	got := make(chan []byte, 1)
	go func() {
		peer := &Client{r: bufio.NewReader(broker)}
		h, body, err := peer.readPacket()
		if err != nil {
			return
		}
		got <- append([]byte{h}, body...)
		if reply != 0 {
			_, _ = broker.Write([]byte{reply << 4, 3, body[0], body[1], code})
		}
	}()
	return got
}

func noop(string, []byte, time.Time) {}

func leftovers(c *Client) (subs, acks int) {
	c.smu.RLock()
	defer c.smu.RUnlock()
	return len(c.subs), len(c.acks)
}

func TestRefusedSubscribeLeavesNothing(t *testing.T) {
	c, broker := pipeClient(t)
	answer(broker, typeSuback, subackFailure)
	if err := c.Subscribe("a/b", noop); err == nil {
		t.Fatal("a refused subscription was accepted")
	}
	if subs, acks := leftovers(c); subs != 0 || acks != 0 {
		t.Errorf("%d handlers and %d acks left", subs, acks)
	}
}

func TestFailedWriteSubscribeLeavesNothing(t *testing.T) {
	c, broker := pipeClient(t)
	broker.Close()
	if err := c.Subscribe("a/b", noop); err == nil {
		t.Fatal("subscribed over a closed connection")
	}
	if subs, acks := leftovers(c); subs != 0 || acks != 0 {
		t.Errorf("%d handlers and %d acks left", subs, acks)
	}
}

func TestUnsubscribe(t *testing.T) {
	c, broker := pipeClient(t)
	answer(broker, typeSuback, 0)
	if err := c.Subscribe("a/b", noop); err != nil {
		t.Fatal(err)
	}
	got := answer(broker, 0, 0)
	if err := c.Unsubscribe("a/b"); err != nil {
		t.Fatal(err)
	}
	if subs, _ := leftovers(c); subs != 0 {
		t.Errorf("%d handlers left", subs)
	}
	pkt := <-got
	if pkt[0] != typeUnsubscribe<<4|subscribeBits || binary.BigEndian.Uint16(pkt[1:]) == 0 || string(pkt[5:]) != "a/b" {
		t.Errorf("UNSUBSCRIBE %x", pkt)
	}
}

func TestSubscribeLiveSkipsRetained(t *testing.T) {
	c, broker := pipeClient(t)
	got := make(chan string, 2)
	go func() {
		peer := &Client{r: bufio.NewReader(broker)}
		_, body, err := peer.readPacket()
		if err != nil {
			return
		}
		_, _ = broker.Write([]byte{typeSuback << 4, 3, body[0], body[1], 0})
		for _, m := range []struct {
			flags   byte
			payload string
		}{{flagRetain, "retained"}, {0, "live"}} {
			pub := append(appendString(nil, "a/b"), m.payload...)
			_, _ = broker.Write(append([]byte{typePublish<<4 | m.flags, byte(len(pub))}, pub...))
		}
	}()
	err := c.SubscribeLive("a/b", func(_ string, p []byte, _ time.Time) { got <- string(p) })
	if err != nil {
		t.Fatal(err)
	}
	if p := <-got; p != "live" {
		t.Errorf("got the %s message first", p)
	}
}
