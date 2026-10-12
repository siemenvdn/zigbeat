package engine

import (
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/show"
)

func TestLateKickIsDroppedItsReturnStays(t *testing.T) {
	e := started(t, "separate", 100)
	e.Music(t0, moment(8)) // At t0+4s: kicks for lamp-1 and lamp-2, breathe for lamp-4
	sendAt := e.queue["lamp-1"][0].sendAt
	got := lightsOf(e.Due(sendAt.Add(200 * time.Millisecond)))
	if got != "lamp-4" {
		t.Fatalf("200 ms late: sent %q, want only the breathe light", got)
	}
	if len(e.queue["lamp-1"]) != 1 || len(e.queue["lamp-2"]) != 1 {
		t.Fatalf("each return stays: %v %v", e.queue["lamp-1"], e.queue["lamp-2"])
	}
	if e.dropped != 2 {
		t.Errorf("two late kicks, counted %d", e.dropped)
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
	e.Music(at(5_000), moment(8)) // At t0+4s, already past
	if len(e.queue["lamp-1"])+len(e.queue["lamp-4"]) != 0 {
		t.Fatal("a beat in the past must not be planned")
	}
}

func TestSetOffsetMovesTheSendTime(t *testing.T) {
	e := started(t, "separate", 100)
	e.Music(t0, moment(8))
	before := e.queue["lamp-1"][0].sendAt
	clear(e.queue)
	e.SetOffset(120 * time.Millisecond)
	e.Music(t0, moment(16))
	if d := e.queue["lamp-1"][0].sendAt.Sub(before); d != 4*time.Second+120*time.Millisecond {
		t.Errorf("8 beats and 120 ms later, got %s", d)
	}
}

func TestClearDropsPlannedMoments(t *testing.T) {
	e := started(t, "separate", 100)
	e.Music(t0, moment(8)) // kicks for lamp-1 and lamp-2, breathe for lamp-4
	e.Music(t0, music.Clear{})
	if len(e.queue["lamp-1"]) != 1 || len(e.queue["lamp-2"]) != 1 || len(e.queue["lamp-4"]) == 0 {
		t.Errorf("a seek drops the planned kicks and keeps their returns and breathe: %v %v %v",
			e.queue["lamp-1"], e.queue["lamp-2"], e.queue["lamp-4"])
	}
	if e.dropped != 0 {
		t.Errorf("forgotten moments are not late ones, counted %d", e.dropped)
	}

	e = started(t, "separate", 100)
	e.Music(t0, moment(8))
	e.Due(e.queue["lamp-1"][0].sendAt) // the kicks leave
	e.Music(t0, music.Clear{})
	if len(e.queue["lamp-1"]) != 1 || len(e.queue["lamp-2"]) != 1 {
		t.Errorf("a kick that left keeps its return: %v %v", e.queue["lamp-1"], e.queue["lamp-2"])
	}
}

// TestBuildPaintsADroppedKickAgain: a light whose kick was dropped does not
// show the colour, so build gives it that colour again.
func TestBuildPaintsADroppedKickAgain(t *testing.T) {
	e := started(t, "separate", 100)
	eff := testConfig("separate").Effects
	eff.Variant = "build"
	e.SetEffects(t0, eff)
	kicked := func() string {
		for l, q := range e.queue {
			for _, m := range q {
				if m.Drop {
					return l
				}
			}
		}
		return ""
	}
	e.Music(t0, moment(8))
	first := kicked()
	e.Due(e.queue[first][0].sendAt.Add(200 * time.Millisecond))
	if kicked() != "" {
		t.Fatal("the late kick was not dropped")
	}
	e.Music(at(4_300), moment(12))
	if got := kicked(); got != first {
		t.Errorf("build painted %q, want %q again", got, first)
	}
}
