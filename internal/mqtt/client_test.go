package mqtt

import (
	"bytes"
	"testing"
	"time"
)

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
