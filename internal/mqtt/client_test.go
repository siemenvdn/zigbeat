package mqtt

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/mqtt/mqtttest"
)

func TestWillAndRetain(t *testing.T) {
	b := mqtttest.Start(t)
	c := dial(t, b, Options{ClientID: "id", Username: "u", Password: "p",
		WillTopic: "zigbeat/availability", WillPayload: []byte("offline")})
	want := append(mqtttest.String("MQTT"), 4, 0x02|0x04|0x20|0x40|0x80, 0, byte(defaultKeepAlive/time.Second))
	for _, s := range []string{"id", "zigbeat/availability", "offline", "u", "p"} {
		want = append(want, mqtttest.String(s)...)
	}
	if p := b.Next(t, 1); !bytes.Equal(p[1:], want) {
		t.Errorf("CONNECT % x, want % x", p[1:], want)
	}
	if err := c.PublishRetained("zigbeat/state", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if p := b.Next(t, 3); p[0] != 0x31 || !bytes.Equal(p[1:], append(mqtttest.String("zigbeat/state"), "{}"...)) {
		t.Errorf("PUBLISH % x", p)
	}
}

func TestDoneOnCloseAndOnADrop(t *testing.T) {
	b := mqtttest.Start(t)
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

	b = mqtttest.Start(t)
	c = dial(t, b, Options{ClientID: "id"})
	conn := <-b.Conn
	conn.Close()
	select {
	case <-c.Done():
	case <-time.After(testWait):
		t.Fatal("Done after the broker dropped the connection")
	}
	if c.Err() == nil {
		t.Error("no error after the broker dropped the connection")
	}
	if b.Dialled(300 * time.Millisecond) {
		t.Error("the client dialled again by itself; the commands do that")
	}
}

// TestDialIgnoresAllProxy.
func TestDialIgnoresAllProxy(t *testing.T) {
	if os.Getenv("ZIGBEAT_PROXY_CHILD") == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestDialIgnoresAllProxy$")
		cmd.Env = append(os.Environ(), "ZIGBEAT_PROXY_CHILD=1", "ALL_PROXY=socks5://127.0.0.1:1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return
	}
	dial(t, mqtttest.Start(t), Options{ClientID: "id"})
}
