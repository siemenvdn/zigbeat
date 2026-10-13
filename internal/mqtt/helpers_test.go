package mqtt

import (
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/mqtt/mqtttest"
)

const testWait = mqtttest.Wait

func dial(t *testing.T, b *mqtttest.Broker, o Options) *Client {
	t.Helper()
	c, err := Dial(b.Addr, o)
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
