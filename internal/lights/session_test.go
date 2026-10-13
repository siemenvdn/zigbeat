package lights

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/mqtt/mqtttest"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

const (
	publishPacket   = 3
	subscribePacket = 8
)

// TestSessionAwaitsReadsAndNoticesADrop drives one session over the fake broker.
func TestSessionAwaitsReadsAndNoticesADrop(t *testing.T) {
	b := mqtttest.Start(t)
	seen := make(chan string, 4)
	s, err := Dial(config.MQTT{Broker: b.Addr, BaseTopic: "zigbee2mqtt"}, "test", Watch{
		Devices: []string{"lamp-1"},
		OnState: func(d string, _ z2m.State, _ time.Time) { seen <- d },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	b.Next(t, subscribePacket)

	type answer struct {
		st z2m.State
		ok bool
	}
	got := make(chan answer, 1)
	since := time.Now()
	go func() {
		st, ok := s.Await(context.Background(), "lamp-1", since, mqtttest.Wait)
		got <- answer{st, ok}
	}()
	b.Publish(t, "zigbee2mqtt/lamp-1", `{"state":"OFF"}`, true)
	b.Publish(t, "zigbee2mqtt/lamp-1", `{"state":"ON","brightness":100}`, false)
	select {
	case a := <-got:
		if !a.ok || a.st.State != "ON" || a.st.Brightness == nil || *a.st.Brightness != 100 {
			t.Fatalf("await got %+v, %v; want the live state", a.st, a.ok)
		}
	case <-time.After(mqtttest.Wait):
		t.Fatal("a live state must wake Await")
	}
	if d := <-seen; d != "lamp-1" || len(seen) != 0 {
		t.Errorf("OnState saw %s and %d more; want lamp-1 once, the retained state skipped", d, len(seen))
	}
	if _, ok := s.Await(context.Background(), "lamp-1", time.Now(), 50*time.Millisecond); ok {
		t.Error("no state after since, so Await must time out")
	}

	read := make(chan bool, 1)
	go func() {
		_, ok := s.Read(context.Background(), "far-lamp", mqtttest.Wait)
		read <- ok
	}()
	b.Next(t, subscribePacket)
	if p := b.Next(t, publishPacket); !bytes.Contains(p, []byte("zigbee2mqtt/far-lamp/get")) {
		t.Fatalf("Read must ask far-lamp for its state, sent %q", p)
	}
	b.Publish(t, "zigbee2mqtt/far-lamp", `{"state":"ON"}`, false)
	if !<-read {
		t.Fatal("Read must return the answer")
	}
	if _, _, ok := s.Last("far-lamp"); !ok || !s.Online() {
		t.Error("the answer is kept and the session is online")
	}

	c := <-b.Conn
	c.Close()
	select {
	case <-s.Done():
	case <-time.After(mqtttest.Wait):
		t.Fatal("Done after the broker dropped the connection")
	}
	if _, ok := s.Await(context.Background(), "lamp-1", time.Now(), mqtttest.Wait); ok || s.Online() {
		t.Error("a cut connection ends every wait and takes the session offline")
	}
}

type fakeReader struct{ answers []time.Duration }

func (f *fakeReader) Read(context.Context, string, time.Duration) (time.Duration, bool) {
	d := f.answers[0]
	f.answers = f.answers[1:]
	return d, d > 0
}

func TestReadTimesKeepsTheAnswersAndStopsOnCancel(t *testing.T) {
	ms := time.Millisecond
	got := ReadTimes(context.Background(), &fakeReader{[]time.Duration{40 * ms, 0, 60 * ms}}, "far-lamp", 3, 0)
	if len(got) != 2 || got[0] != 40*ms || got[1] != 60*ms {
		t.Errorf("read times %v, want the two answers", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := ReadTimes(ctx, &fakeReader{[]time.Duration{40 * ms, 50 * ms}}, "far-lamp", 2, time.Hour); len(got) != 1 {
		t.Errorf("after Ctrl-C the next read is not waited for: %v", got)
	}
}
