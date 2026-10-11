package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/run/show"
)

func TestRisingLatencyBacksOff(t *testing.T) {
	e := New(testConfig("separate"), 0, t0)
	confirm := func(i, latency, frames int) {
		e.fl.Add("lamp-1", at(i*1000), true, frames)
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

func TestPauseForgetsOldLatency(t *testing.T) {
	e := New(testConfig("separate"), 0, t0)
	confirm := func(ms, latency int) {
		e.fl.Add("lamp-1", at(ms), true, 1)
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
	if !e.bo.Tick(at(100_000)) || e.bo.Level() != 2 {
		t.Fatalf("a quiet minute resumes at level 2, level %d", e.bo.Level())
	}
	confirm(101_000, 40)
	if e.bo.Level() != 2 {
		t.Fatalf("the network is fast again, but latency from before the pause paused it again: level %d", e.bo.Level())
	}
}

func TestSlowNetworkNote(t *testing.T) {
	e := New(testConfig("separate"), 0, t0)
	for i := 0; i < 20; i++ {
		e.fl.Add("lamp-1", at(i*1000), true, 1)
		e.State("lamp-1", at(i*1000+200))
	}
	notes := e.TakeNotes()
	if len(notes) != 1 || !strings.Contains(notes[0], "run zigbeat probe again") {
		t.Fatalf("200 ms against a 50 ms budget latency: notes %q", notes)
	}
	e.fl.Add("lamp-1", at(30_000), true, 1)
	e.State("lamp-1", at(30_200))
	if notes := e.TakeNotes(); len(notes) != 0 {
		t.Fatalf("the note is printed once: %q", notes)
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
	e.ZigbeeError(at(0), "Publish 'set' to 'lamp-1' failed: 'Timeout'")
	e.ZigbeeError(at(1_000), "Publish 'set' to 'lamp-10' failed: 'Timeout'")
	e.ZigbeeError(at(2_000), "Publish 'set' to 'lamp-2' failed: 'Timeout'")
	if e.bo.Level() != 1 {
		t.Fatalf("lamp-10 is not ours, so only two named errors: level %d", e.bo.Level())
	}
	e.TakeNotes()
	e.ZigbeeError(at(3_000), "Publish 'set' to 'lamp-3' failed: 'Timeout'")
	if e.bo.Level() != flow.PauseLevel {
		t.Fatalf("three named errors within a minute pause the show: level %d", e.bo.Level())
	}
	if notes := e.TakeNotes(); len(notes) != 1 || !strings.Contains(notes[0], "paused") {
		t.Errorf("notes %q", notes)
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
	e.Beat(t0, moment(8))
	got := e.Due(t0)
	if lightsOf(got) != "far-lamp" {
		t.Fatalf("a pause only reads the canary, sent %q", lightsOf(got))
	}
	e.State("far-lamp", at(100))
	if !e.bo.Tick(at(60_000)) || e.bo.Level() != 2 {
		t.Fatalf("a quiet minute resumes at level 2, level %d", e.bo.Level())
	}
	e.Beat(at(60_000), moment(240)) // 120 s; level 2 moments fall on every 8th beat
	if len(e.queue["lamp-1"])+len(e.queue["lamp-3"]) == 0 {
		t.Fatal("after the pause the show plans moments again")
	}
}
