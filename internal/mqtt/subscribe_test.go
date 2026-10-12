package mqtt

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

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
