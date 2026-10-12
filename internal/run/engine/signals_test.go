package engine

import (
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/run/show"
)

func TestRisingLatencyBacksOff(t *testing.T) {
	e := New(testConfig("separate"), 0, t0)
	confirm := func(i, latency, frames int) {
		e.fl.Add("lamp-1", at(i*1000), frames)
		e.State("lamp-1", at(i*1000+latency))
	}
	for i := 0; i < 20; i++ {
		confirm(i, 80, 2) // 40 ms per frame becomes the baseline
	}
	for i := 20; i < 29; i++ {
		confirm(i, 100, 1)
	}
	if e.bo.Level() != 0 || e.fl.Base() != 40*time.Millisecond {
		t.Fatalf("median still 40 ms: level %d, base %s", e.bo.Level(), e.fl.Base())
	}
	confirm(29, 100, 1)
	if e.bo.Level() != 1 {
		t.Fatalf("median 100 ms is above 2 x 40 ms: level %d", e.bo.Level())
	}
}

// TestBurstsAreNoRisingLatency: messages that leave together wait for each
// other at a coordinator that sends one frame at a time. A moment of four
// one-frame messages after a phrase of single kicks is no slower network.
func TestBurstsAreNoRisingLatency(t *testing.T) {
	e := New(testConfig("separate"), 0, t0)
	const service = 90 // ms per frame, as fake-z2m's default
	for i := 0; i < 20; i++ {
		e.fl.Add("lamp-1", at(i*1000), 2) // a kick alone sets the baseline
		e.State("lamp-1", at(i*1000+2*service))
	}
	burst := func(i, service int) {
		ls := []string{"lamp-1", "lamp-2", "lamp-3", "lamp-4"}
		for _, l := range ls {
			e.fl.Add(l, at(i*1000), 1)
		}
		for k, l := range ls {
			e.State(l, at(i*1000+(k+1)*service))
		}
	}
	for i := 20; i < 40; i++ {
		burst(i, service)
	}
	if e.bo.Level() != 0 {
		t.Fatalf("the same coordinator, only busier moments: level %d", e.bo.Level())
	}
	for i := 40; i < 60; i++ {
		burst(i, 3*service)
	}
	if e.bo.Level() == 0 {
		t.Fatal("a coordinator three times slower under the same moments backs off")
	}
}

func TestPauseForgetsOldLatency(t *testing.T) {
	e := New(testConfig("separate"), 0, t0)
	confirm := func(ms, latency int) {
		e.fl.Add("lamp-1", at(ms), 1)
		e.State("lamp-1", at(ms+latency))
	}
	for i := 0; i < 40; i++ {
		latency := 40
		if i >= 20 {
			latency = 200
		}
		confirm(i*1000, latency)
	}
	e.bo.Pause(at(40_000))
	e.paused(at(40_000), "test")
	if !e.bo.Tick(at(100_000)) || e.bo.Level() != flow.PauseLevel-1 {
		t.Fatalf("a quiet minute resumes one level below the pause, level %d", e.bo.Level())
	}
	confirm(101_000, 40)
	if e.bo.Level() != flow.PauseLevel-1 {
		t.Fatalf("the network is fast again, but latency from before the pause paused it again: level %d", e.bo.Level())
	}
}

func TestZigbeeErrors(t *testing.T) {
	e := New(testConfig("separate"), 0, t0)
	for i := 0; i < 4; i++ {
		e.ZigbeeError(at(i*20_000), "Publish 'set' to 'battery-sensor' failed: 'Timeout'")
	}
	if e.bo.Level() != 1 {
		t.Fatalf("errors elsewhere may only reach level 1, got %d", e.bo.Level())
	}

	e = New(testConfig("separate"), 0, t0)
	for i, l := range []string{"lamp-1", "lamp-10", "lamp-2", "lamp-3"} {
		e.ZigbeeError(at(i*1_000), "Publish 'set' to '"+l+"' failed: 'Timeout'")
	}
	if e.bo.Level() != 1 {
		t.Fatalf("errors about our lights raise one step per 10 s and never pause at once: level %d", e.bo.Level())
	}
	e.ZigbeeError(at(10_000), "Publish 'set' to 'lamp-1' failed: 'Timeout'")
	if e.bo.Level() != 2 {
		t.Fatalf("10 s later the next step: level %d", e.bo.Level())
	}
}

func TestZigbeeErrorKeepsTheSlot(t *testing.T) {
	e := started(t, "separate", 100)
	land := at(10_000)
	e.enqueue([]show.Message{{Light: "lamp-1", Payload: map[string]any{"brightness": 10}, Land: land}})
	e.Due(land)
	// The error may be about an older message; only the echo or the 5 s expiry frees the slot.
	e.ZigbeeError(land.Add(time.Second), "Publish 'set' to 'lamp-1' failed: 'Timeout'")
	if !e.fl.Busy("lamp-1") || e.fails["lamp-1"] != 0 {
		t.Fatalf("busy %v, fails %d", e.fl.Busy("lamp-1"), e.fails["lamp-1"])
	}
}

func TestPauseSendsOnlyCanaryAndResumes(t *testing.T) {
	cfg := testConfig("separate")
	cfg.Canary = "far-lamp"
	e := New(cfg, time.Second, t0)
	e.bo.Pause(t0)
	e.paused(t0, "test")
	e.Music(t0, moment(8))
	got := e.Due(t0)
	if lightsOf(got) != "far-lamp" {
		t.Fatalf("a pause only reads the canary, sent %q", lightsOf(got))
	}
	e.State("far-lamp", at(100))
	if !e.bo.Tick(at(60_000)) || e.bo.Level() != flow.PauseLevel-1 {
		t.Fatalf("a quiet minute resumes one level below the pause, level %d", e.bo.Level())
	}
	e.Music(at(60_000), moment(240)) // 120 s, on a bar
	if len(e.queue["lamp-1"])+len(e.queue["lamp-3"]) == 0 {
		t.Fatal("after the pause the show plans moments again")
	}
}
