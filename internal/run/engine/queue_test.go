package engine

import (
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/run/show"
)

func TestLateMomentIsDroppedWithItsReturn(t *testing.T) {
	e := started(t, "separate", 100)
	e.Beat(t0, moment(8)) // At t0+4s: kicks for lamp-1 and lamp-2, breathe for lamp-4
	sendAt := e.queue["lamp-1"][0].sendAt
	got := lightsOf(e.Due(sendAt.Add(200 * time.Millisecond)))
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
	e.Beat(t0, moment(8))
	sendAt := e.queue["lamp-1"][0].sendAt
	e.lim = flow.NewLimiter(5, sendAt)
	e.lim.Take(sendAt, flow.Burst-2) // room for one kick of 2 frames
	if got := lightsOf(e.Due(sendAt)); got != "lamp-1" {
		t.Fatalf("tokens for one kick: sent %q", got)
	}
	e.State("lamp-1", sendAt.Add(40*time.Millisecond))
	if got := lightsOf(e.Due(sendAt.Add(400 * time.Millisecond))); got != "lamp-2" {
		t.Fatalf("400 ms late, the unit already started: sent %q, want lamp-2", got)
	}
	if len(e.queue["lamp-3"]) == 0 {
		t.Fatal("lamp-3 of a started unit must not be dropped")
	}
}

func TestDueMessagesMerge(t *testing.T) {
	e := started(t, "separate", 100)
	land := at(10_000)
	e.enqueue([]show.Message{
		{Light: "lamp-1", Payload: map[string]any{"brightness": 152, "transition": 0.3}, Land: land},
		{Light: "lamp-1", Payload: map[string]any{"color": map[string]string{"hex": "#2962ff"}, "transition": 0.7}, Land: land.Add(10 * time.Millisecond)},
	})
	got := e.Due(land.Add(20 * time.Millisecond))
	if len(got) != 1 {
		t.Fatalf("want one merged message, got %d", len(got))
	}
	p := got[0].Payload
	if p["brightness"] != 152 || p["color"] == nil || p["transition"] != 0.7 {
		t.Errorf("merged payload %v", p)
	}
}

func TestStaleBeatIsSkipped(t *testing.T) {
	e := started(t, "separate", 100)
	e.Beat(at(5_000), moment(8)) // At t0+4s, already past
	if len(e.queue["lamp-1"])+len(e.queue["lamp-4"]) != 0 {
		t.Fatal("a beat in the past must not be planned")
	}
}

func TestSetOffsetMovesTheSendTime(t *testing.T) {
	e := started(t, "separate", 100)
	e.Beat(t0, moment(8))
	before := e.queue["lamp-1"][0].sendAt
	clear(e.queue)
	e.SetOffset(120 * time.Millisecond)
	e.Beat(t0, moment(16))
	if d := e.queue["lamp-1"][0].sendAt.Sub(before); d != 4*time.Second+120*time.Millisecond {
		t.Errorf("8 beats and 120 ms later, got %s", d)
	}
}
