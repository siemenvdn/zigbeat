package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/run/show"
)

func TestStartPassRespectsInFlightLimits(t *testing.T) {
	e := New(testConfig("separate"), 0, t0)
	if got := lightsOf(e.Due(t0)); got != "lamp-1,lamp-2,lamp-3" {
		t.Fatalf("first sends %q, want three lights", got)
	}
	if got := e.Due(at(1)); len(got) != 0 {
		t.Fatalf("a fourth message must wait for a confirmation, sent %q", lightsOf(got))
	}
	e.enqueue([]show.Message{{Light: "lamp-1", Payload: map[string]any{"brightness": 10}, Land: t0}})
	e.State("lamp-2", at(40))
	if got := lightsOf(e.Due(at(41))); got != "lamp-4" {
		t.Fatalf("after lamp-2 answered: sent %q, want lamp-4 (lamp-1 is still busy)", got)
	}
	e.State("lamp-1", at(50))
	if got := lightsOf(e.Due(at(51))); got != "lamp-1" {
		t.Fatalf("lamp-1 is free again: sent %q", got)
	}
}

func TestMoveFreesAfterOneSecond(t *testing.T) {
	e := started(t, "separate", 100)
	land := at(10_000)
	e.enqueue([]show.Message{{Light: "lamp-4", Payload: map[string]any{"brightness_move": 40}, Land: land}})
	if got := lightsOf(e.Due(land)); got != "lamp-4" {
		t.Fatalf("sent %q", got)
	}
	e.Due(land.Add(999 * time.Millisecond))
	if !e.fl.Busy("lamp-4") {
		t.Fatal("a move holds its slot for 1 s")
	}
	e.Due(land.Add(time.Second))
	if e.fl.Busy("lamp-4") || e.bo.Level() != 0 {
		t.Fatalf("after 1 s the slot is free without backoff (level %d)", e.bo.Level())
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
