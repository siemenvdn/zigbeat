package mqtt

import (
	"bufio"
	"bytes"
	"net"
	"testing"
	"time"
)

func TestWillAndRetain(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	type packet struct {
		h    byte
		body []byte
	}
	got := make(chan packet, 2)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		peer := &Client{r: bufio.NewReader(conn)}
		for i := 0; i < 2; i++ {
			h, body, err := peer.readPacket()
			if err != nil {
				return
			}
			got <- packet{h, body}
			if i == 0 {
				_, _ = conn.Write([]byte{typeConnack << 4, 2, 0, 0})
			}
		}
	}()

	c, err := Dial(ln.Addr().String(), Options{ClientID: "id", Username: "u", Password: "p",
		WillTopic: "zigbeat/availability", WillPayload: []byte("offline")})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.PublishRetained("zigbeat/state", []byte("{}")); err != nil {
		t.Fatal(err)
	}

	connect := <-got
	want := appendString(nil, "MQTT")
	want = append(want, protocolLevel, 0x02|0x04|0x20|0x40|0x80, 0, byte(defaultKeepAlive/time.Second))
	for _, s := range []string{"id", "zigbeat/availability", "offline", "u", "p"} {
		want = appendString(want, s)
	}
	if connect.h != typeConnect<<4 || !bytes.Equal(connect.body, want) {
		t.Errorf("CONNECT %x %x, want %x", connect.h, connect.body, want)
	}
	pub := <-got
	if pub.h != typePublish<<4|0x01 || !bytes.Equal(pub.body, append(appendString(nil, "zigbeat/state"), "{}"...)) {
		t.Errorf("PUBLISH %x %q", pub.h, pub.body)
	}
}
