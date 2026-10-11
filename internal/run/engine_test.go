package run

import (
	"strings"
	"testing"
	"time"
)

func lightsOf(ss []send) string {
	var ls []string
	for _, s := range ss {
		ls = append(ls, s.light)
	}
	return strings.Join(ls, ",")
}

// started returns an engine whose start pass went out and was confirmed after 40 ms.
func started(t *testing.T, mode string, maxRate float64) *engine {
	t.Helper()
	cfg := testConfig(mode)
	cfg.Budget.MaxRate = maxRate
	e := newEngine(cfg, 0, t0)
	for i := 0; i < 1000 && len(e.queue["lamp-4"])+len(e.queue["lamp-1"])+len(e.queue["lamp-2"])+len(e.queue["lamp-3"]) > 0; i++ {
		now := at(i * 50)
		for _, s := range e.due(now) {
			e.state(s.light, now.Add(40*time.Millisecond))
		}
	}
	if e.inFlight() != 0 || e.bo.level != 0 {
		t.Fatalf("start pass did not settle: in flight %d, backoff %d", e.inFlight(), e.bo.level)
	}
	return e
}

func TestStartPassRespectsInFlightLimits(t *testing.T) {
	e := newEngine(testConfig("separate"), 0, t0)
	if got := lightsOf(e.due(t0)); got != "lamp-1,lamp-2,lamp-3" {
		t.Fatalf("first sends %q, want three lights", got)
	}
	if got := e.due(at(1)); len(got) != 0 {
		t.Fatalf("a fourth message must wait for a confirmation, sent %q", lightsOf(got))
	}
	e.enqueue([]planned{{light: "lamp-1", payload: map[string]any{"brightness": 10}, land: t0}})
	e.state("lamp-2", at(40))
	if got := lightsOf(e.due(at(41))); got != "lamp-4" {
		t.Fatalf("after lamp-2 answered: sent %q, want lamp-4 (lamp-1 is still busy)", got)
	}
	e.state("lamp-1", at(50))
	if got := lightsOf(e.due(at(51))); got != "lamp-1" {
		t.Fatalf("lamp-1 is free again: sent %q", got)
	}
}

func TestLateMomentIsDroppedWithItsReturn(t *testing.T) {
	e := started(t, "separate", 100)
	e.beat(t0, moment(8)) // At t0+4s: kicks for lamp-1 and lamp-2, breathe for lamp-4
	sendAt := e.queue["lamp-1"][0].sendAt
	got := lightsOf(e.due(sendAt.Add(200 * time.Millisecond)))
	if got != "lamp-4" {
		t.Fatalf("200 ms late: sent %q, want only the breathe light", got)
	}
	if len(e.queue["lamp-1"])+len(e.queue["lamp-2"]) != 0 {
		t.Fatalf("returns of a dropped unit must go too: %v %v", e.queue["lamp-1"], e.queue["lamp-2"])
	}
	if e.dropped != 2 {
		t.Errorf("two separate units dropped, counted %d", e.dropped)
	}
}

func TestTogetherUnitIsNeverSplit(t *testing.T) {
	e := started(t, "together", 5)
	e.beat(t0, moment(8))
	sendAt := e.queue["lamp-1"][0].sendAt
	e.lim.tokens, e.lim.last = 2, sendAt
	if got := lightsOf(e.due(sendAt)); got != "lamp-1" {
		t.Fatalf("tokens for one kick: sent %q", got)
	}
	e.state("lamp-1", sendAt.Add(40*time.Millisecond))
	if got := lightsOf(e.due(sendAt.Add(400 * time.Millisecond))); got != "lamp-2" {
		t.Fatalf("400 ms late, the unit already started: sent %q, want lamp-2", got)
	}
	if len(e.queue["lamp-3"]) == 0 {
		t.Fatal("lamp-3 of a started unit must not be dropped")
	}
}

func TestDueMessagesMerge(t *testing.T) {
	e := started(t, "separate", 100)
	land := at(10_000)
	e.enqueue([]planned{
		{light: "lamp-1", payload: map[string]any{"brightness": 152, "transition": 0.3}, land: land},
		{light: "lamp-1", payload: map[string]any{"color": map[string]string{"hex": "#2962ff"}, "transition": 0.7}, land: land.Add(10 * time.Millisecond)},
	})
	got := e.due(land.Add(20 * time.Millisecond))
	if len(got) != 1 {
		t.Fatalf("want one merged message, got %d", len(got))
	}
	p := got[0].payload
	if p["brightness"] != 152 || p["color"] == nil || p["transition"] != 0.7 {
		t.Errorf("merged payload %v", p)
	}
}

func TestMoveFreesAfterOneSecond(t *testing.T) {
	e := started(t, "separate", 100)
	land := at(10_000)
	e.enqueue([]planned{{light: "lamp-4", payload: map[string]any{"brightness_move": 40}, land: land}})
	if got := lightsOf(e.due(land)); got != "lamp-4" {
		t.Fatalf("sent %q", got)
	}
	e.due(land.Add(999 * time.Millisecond))
	if !e.fl.busy("lamp-4") {
		t.Fatal("a move holds its slot for 1 s")
	}
	e.due(land.Add(time.Second))
	if e.fl.busy("lamp-4") || e.bo.level != 0 {
		t.Fatalf("after 1 s the slot is free without backoff (level %d)", e.bo.level)
	}
}

func TestLostMessageBacksOff(t *testing.T) {
	e := started(t, "separate", 100)
	land := at(10_000)
	e.enqueue([]planned{{light: "lamp-1", payload: map[string]any{"brightness": 10}, land: land}})
	e.due(land)
	e.due(land.Add(5 * time.Second))
	if e.bo.level != 1 || e.fl.busy("lamp-1") {
		t.Fatalf("lost after 5 s: level %d, busy %v", e.bo.level, e.fl.busy("lamp-1"))
	}
	if notes := e.takeNotes(); len(notes) != 1 || !strings.Contains(notes[0], "lamp-1 did not answer") {
		t.Errorf("notes %q", notes)
	}
}

func TestRisingLatencyBacksOff(t *testing.T) {
	e := newEngine(testConfig("separate"), 0, t0)
	confirm := func(i, latency, frames int) {
		e.fl.add("lamp-1", at(i*1000), true, frames)
		e.state("lamp-1", at(i*1000+latency))
	}
	for i := 0; i < 20; i++ {
		confirm(i, 80, 2) // 40 ms per frame becomes the baseline
	}
	for i := 20; i < 29; i++ {
		confirm(i, 100, 1)
	}
	if e.bo.level != 0 || e.fl.base != 40*time.Millisecond {
		t.Fatalf("median still 40 ms: level %d, base %s", e.bo.level, e.fl.base)
	}
	confirm(29, 100, 1)
	if e.bo.level != 1 {
		t.Fatalf("median 100 ms is above 2 x 40 ms: level %d", e.bo.level)
	}
}

func TestPauseForgetsOldLatency(t *testing.T) {
	e := newEngine(testConfig("separate"), 0, t0)
	confirm := func(ms, latency int) {
		e.fl.add("lamp-1", at(ms), true, 1)
		e.state("lamp-1", at(ms+latency))
	}
	for i := 0; i < 40; i++ {
		latency := 40
		if i >= 20 {
			latency = 200
		}
		confirm(i*1000, latency)
	}
	e.bo.pause(at(40_000))
	e.paused(at(40_000), "test")
	if !e.bo.tick(at(100_000)) || e.bo.level != 2 {
		t.Fatalf("a quiet minute resumes at level 2, level %d", e.bo.level)
	}
	confirm(101_000, 40)
	if e.bo.level != 2 {
		t.Fatalf("the network is fast again, but latency from before the pause paused it again: level %d", e.bo.level)
	}
}

func TestSlowNetworkNote(t *testing.T) {
	e := newEngine(testConfig("separate"), 0, t0)
	for i := 0; i < 20; i++ {
		e.fl.add("lamp-1", at(i*1000), true, 1)
		e.state("lamp-1", at(i*1000+200))
	}
	notes := e.takeNotes()
	if len(notes) != 1 || !strings.Contains(notes[0], "run zigbeat probe again") {
		t.Fatalf("200 ms against a 50 ms budget latency: notes %q", notes)
	}
	e.fl.add("lamp-1", at(30_000), true, 1)
	e.state("lamp-1", at(30_200))
	if notes := e.takeNotes(); len(notes) != 0 {
		t.Fatalf("the note is printed once: %q", notes)
	}
}

func TestZigbeeErrors(t *testing.T) {
	e := newEngine(testConfig("separate"), 0, t0)
	for i := 0; i < 4; i++ {
		e.zigbeeError(at(i*20_000), "Publish 'set' to 'battery-sensor' failed: 'Timeout'")
	}
	if e.bo.level != 1 {
		t.Fatalf("errors elsewhere may only reach level 1, got %d", e.bo.level)
	}

	e = newEngine(testConfig("separate"), 0, t0)
	e.zigbeeError(at(0), "Publish 'set' to 'lamp-1' failed: 'Timeout'")
	e.zigbeeError(at(1_000), "Publish 'set' to 'lamp-10' failed: 'Timeout'")
	e.zigbeeError(at(2_000), "Publish 'set' to 'lamp-2' failed: 'Timeout'")
	if e.bo.level != 1 {
		t.Fatalf("lamp-10 is not ours, so only two named errors: level %d", e.bo.level)
	}
	e.takeNotes()
	e.zigbeeError(at(3_000), "Publish 'set' to 'lamp-3' failed: 'Timeout'")
	if e.bo.level != pauseLevel {
		t.Fatalf("three named errors within a minute pause the show: level %d", e.bo.level)
	}
	if notes := e.takeNotes(); len(notes) != 1 || !strings.Contains(notes[0], "paused") {
		t.Errorf("notes %q", notes)
	}
}

func TestPauseSendsOnlyCanaryAndResumes(t *testing.T) {
	cfg := testConfig("separate")
	cfg.Canary = "far-lamp"
	e := newEngine(cfg, time.Second, t0)
	e.bo.pause(t0)
	e.paused(t0, "test")
	e.beat(t0, moment(8))
	got := e.due(t0)
	if lightsOf(got) != "far-lamp" {
		t.Fatalf("a pause only reads the canary, sent %q", lightsOf(got))
	}
	e.state("far-lamp", at(100))
	if !e.bo.tick(at(60_000)) || e.bo.level != 2 {
		t.Fatalf("a quiet minute resumes at level 2, level %d", e.bo.level)
	}
	e.beat(at(60_000), moment(240)) // 120 s; level 2 moments fall on every 8th beat
	if len(e.queue["lamp-1"])+len(e.queue["lamp-3"]) == 0 {
		t.Fatal("after the pause the show plans moments again")
	}
}

func TestStaleBeatIsSkipped(t *testing.T) {
	e := started(t, "separate", 100)
	e.beat(at(5_000), moment(8)) // At t0+4s, already past
	if len(e.queue["lamp-1"])+len(e.queue["lamp-4"]) != 0 {
		t.Fatal("a beat in the past must not be planned")
	}
}

func TestUnreachableLightIsLeftAlone(t *testing.T) {
	e := started(t, "separate", 100)
	lose := func(ms int) {
		e.enqueue([]planned{{light: "lamp-3", payload: map[string]any{"brightness": 10}, land: at(ms)}})
		if got := lightsOf(e.due(at(ms))); got != "lamp-3" {
			t.Fatalf("at %d ms sent %q, want lamp-3", ms, got)
		}
		e.due(at(ms + 5_000))
	}
	lose(10_000)
	if e.bo.level != 1 {
		t.Fatalf("the first loss may be the network: level %d", e.bo.level)
	}
	lose(30_000)
	if _, gone := e.away["lamp-3"]; !gone || e.bo.level != 1 {
		t.Fatalf("second loss in a row: away %v, level %d", e.away, e.bo.level)
	}
	e.beat(at(36_000), moment(80))
	if len(e.queue["lamp-3"]) != 0 {
		t.Fatal("nothing is planned for a light that is left alone")
	}
	got := e.due(at(95_000))
	if len(got) == 0 || got[0].light != "lamp-3" || !got[0].get || strings.Count(lightsOf(got), "lamp-3") != 1 {
		t.Fatalf("a minute later it is read once: %+v", got)
	}
	e.state("lamp-3", at(95_040))
	if _, gone := e.away["lamp-3"]; gone || e.fails["lamp-3"] != 0 {
		t.Fatal("an answer brings it back")
	}
}

func TestCanary(t *testing.T) {
	cfg := testConfig("separate")
	cfg.Canary = "far-lamp"
	e := newEngine(cfg, 200*time.Millisecond, t0)
	got := e.due(t0)
	if len(got) == 0 || got[0].light != "far-lamp" || !got[0].get {
		t.Fatalf("first read goes to the canary: %+v", got)
	}
	if len(got) != 3 {
		t.Fatalf("the canary read takes one of the 3 slots: %q", lightsOf(got))
	}
	e.state("far-lamp", at(300))
	if e.bo.level != 1 || e.canary.last != 300*time.Millisecond {
		t.Fatalf("300 ms is above the 200 ms limit: level %d", e.bo.level)
	}
	for _, s := range e.due(at(4_999)) {
		if s.light == "far-lamp" {
			t.Fatal("the next read waits 5 s")
		}
	}
}

func TestForeignStatesAreHarmless(t *testing.T) {
	e := started(t, "separate", 100)
	e.state("lamp-1", at(20_000)) // Home Assistant changed a light that has nothing in flight
	e.state("hallway", at(20_000))
	if e.inFlight() != 0 || e.bo.level != 0 || len(e.fl.lat) != 4 {
		t.Fatalf("in flight %d, level %d, samples %d", e.inFlight(), e.bo.level, len(e.fl.lat))
	}
}

func TestHalt(t *testing.T) {
	e := newEngine(testConfig("separate"), 0, t0)
	e.due(t0)
	e.halt()
	e.beat(t0, moment(8))
	if e.due(at(5_000)) != nil {
		t.Fatal("a halted engine sends nothing")
	}
	e.state("lamp-1", at(40))
	if e.fl.busy("lamp-1") {
		t.Fatal("messages in flight can still be confirmed after halt")
	}
}

func TestStatus(t *testing.T) {
	e := started(t, "separate", 100)
	line := e.status(at(10_000))
	for _, want := range []string{"moments 0", "dropped 0", "in flight 0", "p50 20 ms", "backoff 0", "canary -", "z2m err 0"} {
		if !strings.Contains(line, want) {
			t.Errorf("status %q lacks %q", line, want)
		}
	}
}
