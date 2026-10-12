package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/run/show"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// TestStartPassRespectsInFlightLimits: six lights may have a message in flight,
// one full moment; the seventh waits for a confirmation.
func TestStartPassRespectsInFlightLimits(t *testing.T) {
	cfg := testConfig("separate")
	cfg.Fixtures[1].Lights = []string{"lamp-4", "lamp-5", "lamp-6", "lamp-7"}
	e := New(cfg, 0, t0)
	if got := lightsOf(e.Due(t0)); got != "lamp-1,lamp-2,lamp-3,lamp-4,lamp-5,lamp-6" {
		t.Fatalf("first sends %q, want six lights", got)
	}
	if got := e.Due(at(1)); len(got) != 0 {
		t.Fatalf("a seventh message must wait for a confirmation, sent %q", lightsOf(got))
	}
	e.enqueue([]show.Message{{Light: "lamp-1", Payload: map[string]any{"brightness": 10}, Land: t0}})
	e.State("lamp-2", at(40))
	if got := lightsOf(e.Due(at(41))); got != "lamp-7" {
		t.Fatalf("after lamp-2 answered: sent %q, want lamp-7 (lamp-1 is still busy)", got)
	}
	e.State("lamp-1", at(50))
	if got := lightsOf(e.Due(at(51))); got != "lamp-1" {
		t.Fatalf("lamp-1 is free again: sent %q", got)
	}
}

// TestAMomentLeavesTogether: the three kicks and the two pulses of a full
// moment are all in flight at once and leave before their beat; none waits
// for another's confirmation.
func TestAMomentLeavesTogether(t *testing.T) {
	cfg := testConfig("together")
	cfg.Fixtures[1].Lights = []string{"lamp-4", "lamp-5"}
	e := startedWith(t, cfg)
	b := moment(8)
	e.Music(b.At.Add(-music.Lookahead), b) // three kicks and two pulses up, in the first phrase
	sent := map[string]time.Time{}
	most := 0
	for now := b.At.Add(-music.Lookahead); !now.After(b.At); now = now.Add(time.Millisecond) {
		for _, s := range e.Due(now) {
			if _, ok := sent[s.Light]; !ok {
				sent[s.Light] = now
			}
		}
		most = max(most, e.InFlight())
	}
	if most != 5 || len(sent) != 5 || e.dropped != 0 {
		t.Fatalf("in flight at most %d, sent %v, dropped %d", most, sent, e.dropped)
	}
}

func TestLostMessageBacksOff(t *testing.T) {
	e := started(t, "separate", 100)
	land := at(10_000)
	e.enqueue([]show.Message{{Light: "lamp-1", Payload: map[string]any{"brightness": 10}, Land: land}})
	e.Due(land)
	e.Due(land.Add(5 * time.Second))
	if e.bo.Level() != 1 || e.fl.Busy("lamp-1") {
		t.Fatalf("lost after 5 s: level %d, busy %v", e.bo.Level(), e.fl.Busy("lamp-1"))
	}
	if notes := e.TakeNotes(); len(notes) != 1 || !strings.Contains(notes[0], "lamp-1 did not answer") {
		t.Errorf("notes %q", notes)
	}
}

func TestForeignStatesAreHarmless(t *testing.T) {
	e := started(t, "separate", 100)
	e.State("lamp-1", at(20_000)) // Home Assistant changed a light that has nothing in flight
	e.State("hallway", at(20_000))
	if _, n := e.fl.Median(); e.InFlight() != 0 || e.bo.Level() != 0 || n != 4 {
		t.Fatalf("in flight %d, level %d, samples %d", e.InFlight(), e.bo.Level(), n)
	}
}

func TestHalt(t *testing.T) {
	e := New(testConfig("separate"), 0, t0)
	e.Due(t0)
	e.Halt()
	e.Music(t0, moment(8))
	if e.Due(at(5_000)) != nil {
		t.Fatal("a halted engine sends nothing")
	}
	e.State("lamp-1", at(40))
	if e.fl.Busy("lamp-1") {
		t.Fatal("messages in flight can still be confirmed after halt")
	}
}

// TestFramesStayWithinTheBudget: with more to send than max_rate allows, the
// engine never sends more than one moment's burst plus max_rate frames per
// second, and does use the budget.
func TestFramesStayWithinTheBudget(t *testing.T) {
	const rate = 2
	e := started(t, "together", rate)
	from := at(2_000)
	frames, beat := 0, 4
	for now := from; now.Before(at(42_000)); now = now.Add(10 * time.Millisecond) {
		for ; moment(beat).At.Sub(now) <= time.Second; beat++ {
			e.Music(now, moment(beat))
		}
		for _, s := range e.Due(now) {
			frames += z2m.Frames(s.Payload)
			e.State(s.Light, now.Add(time.Millisecond))
		}
		if limit := flow.Burst + rate*now.Sub(from).Seconds(); float64(frames) > limit {
			t.Fatalf("%d frames by %s, more than %g", frames, now.Sub(from), limit)
		}
	}
	if frames < rate*40/2 {
		t.Errorf("%d frames in 40 s at %d frames/s", frames, rate)
	}
}
