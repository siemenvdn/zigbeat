package mqtt

import (
	"bytes"
	"os"
	"os/exec"
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
	_ = b.ln.SetDeadline(time.Now().Add(300 * time.Millisecond)) // paho's own reconnect would dial at once
	if again, err := b.ln.Accept(); err == nil {
		again.Close()
		t.Error("the client dialled again by itself; the commands do that")
	}
}

// TestDialIgnoresAllProxy: the broker is dialled directly, also with ALL_PROXY
// in the environment. x/net reads it once per process, so a child process
// runs the dial.
func TestDialIgnoresAllProxy(t *testing.T) {
	if os.Getenv("ZIGBEAT_PROXY_CHILD") == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestDialIgnoresAllProxy$")
		cmd.Env = append(os.Environ(), "ZIGBEAT_PROXY_CHILD=1", "ALL_PROXY=socks5://127.0.0.1:1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return
	}
	dial(t, startBroker(t), Options{ClientID: "id"})
}
