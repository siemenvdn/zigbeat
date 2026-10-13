package z2m

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/mqtt"
)

func tinyBroker(t *testing.T, retained string) (addr string, unsubs chan string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	unsubs = make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		for {
			h, err := r.ReadByte()
			if err != nil {
				return
			}
			n, _ := r.ReadByte()
			body := make([]byte, n)
			if _, err := io.ReadFull(r, body); err != nil {
				return
			}
			filter := func() string { return string(body[4 : 4+binary.BigEndian.Uint16(body[2:])]) }
			switch h >> 4 {
			case 1:
				_, _ = conn.Write([]byte{0x20, 2, 0, 0})
			case 8:
				topic := filter()
				_, _ = conn.Write([]byte{0x90, 3, body[0], body[1], 0})
				pub := binary.BigEndian.AppendUint16(nil, uint16(len(topic)))
				pub = append(append(pub, topic...), retained...)
				_, _ = conn.Write(append([]byte{0x31, byte(len(pub))}, pub...))
			case 10:
				unsubs <- filter()
				_, _ = conn.Write([]byte{0xb0, 2, body[0], body[1]})
			}
		}
	}()
	return ln.Addr().String(), unsubs
}

func TestDevicesUnsubscribes(t *testing.T) {
	addr, unsubs := tinyBroker(t, `[{"friendly_name":"lamp-1"}]`)
	c, err := mqtt.Dial(addr, mqtt.Options{ClientID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	devs, err := New(c, DefaultBaseTopic).Devices(context.Background())
	if err != nil || len(devs) != 1 || devs[0].Name != "lamp-1" {
		t.Fatalf("devices %v, err %v", devs, err)
	}
	select {
	case f := <-unsubs:
		if f != "zigbee2mqtt/bridge/devices" {
			t.Errorf("unsubscribed from %q", f)
		}
	case <-time.After(time.Second):
		t.Error("bridge/devices is still subscribed")
	}
}

func TestDevicesReportsBadJSON(t *testing.T) {
	addr, _ := tinyBroker(t, `{"oops":1}`)
	c, err := mqtt.Dial(addr, mqtt.Options{ClientID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := New(c, DefaultBaseTopic).Devices(context.Background()); err == nil {
		t.Error("a bridge/devices that is not a list was accepted")
	}
}
