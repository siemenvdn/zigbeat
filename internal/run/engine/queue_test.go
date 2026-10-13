package engine

import (
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/run/show"
)

func TestLateKickIsDroppedItsReturnStays(t *testing.T) {
	e := started(t, "separate", 100)
	e.Music(t0, moment(8))
	sendAt := e.lights["lamp-1"].queue[0].sendAt
	got := lightsOf(e.Due(sendAt.Add(200 * time.Millisecond)))
	if got != "" {
		t.Fatalf("200 ms late: sent %q, want nothing: the pulse up is late too", got)
	}
	if len(e.lights["lamp-1"].queue) != 1 || len(e.lights["lamp-2"].queue) != 1 {
		t.Fatalf("each return stays: %v %v", e.lights["lamp-1"].queue, e.lights["lamp-2"].queue)
	}
	if e.dropped != 3 {
		t.Errorf("two late kicks and the pulse up, counted %d", e.dropped)
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
	e.Music(at(5_000), moment(8))
	if len(e.lights["lamp-1"].queue)+len(e.lights["lamp-4"].queue) != 0 {
		t.Fatal("a beat in the past must not be planned")
	}
}

func TestSetOffsetMovesTheSendTime(t *testing.T) {
	e := started(t, "separate", 100)
	e.Music(t0, moment(8))
	before := e.lights["lamp-1"].queue[0].sendAt
	for _, l := range e.lights {
		l.queue = nil
	}
	e.SetOffset(120 * time.Millisecond)
	e.Music(t0, moment(16))
	if d := e.lights["lamp-1"].queue[0].sendAt.Sub(before); d != 4*time.Second+120*time.Millisecond {
		t.Errorf("8 beats and 120 ms later, got %s", d)
	}
}

func TestClearDropsPlannedMoments(t *testing.T) {
	e := started(t, "separate", 100)
	e.Music(t0, moment(8))
	e.Music(t0, music.Clear{})
	if len(e.lights["lamp-1"].queue) != 1 || len(e.lights["lamp-2"].queue) != 1 || len(e.lights["lamp-4"].queue) == 0 {
		t.Errorf("a seek drops the planned kicks and keeps their returns and the pulse's fall: %v %v %v",
			e.lights["lamp-1"].queue, e.lights["lamp-2"].queue, e.lights["lamp-4"].queue)
	}
	if e.dropped != 0 {
		t.Errorf("forgotten moments are not late ones, counted %d", e.dropped)
	}

	e = started(t, "separate", 100)
	e.Music(t0, moment(8))
	e.Due(e.lights["lamp-1"].queue[0].sendAt)
	e.Music(t0, music.Clear{})
	if len(e.lights["lamp-1"].queue) != 1 || len(e.lights["lamp-2"].queue) != 1 {
		t.Errorf("a kick that left keeps its return: %v %v", e.lights["lamp-1"].queue, e.lights["lamp-2"].queue)
	}
}

// TestBuildPaintsADroppedKickAgain.
func TestBuildPaintsADroppedKickAgain(t *testing.T) {
	e := started(t, "separate", 100)
	eff := testConfig("separate").Effects
	eff.Variant = "build"
	e.SetEffects(t0, eff)
	e.Music(t0, moment(8))
	first := kicked(e)
	e.Due(e.lights[first].queue[0].sendAt.Add(200 * time.Millisecond))
	if kicked(e) != "" {
		t.Fatal("the late kick was not dropped")
	}
	e.Music(at(4_300), moment(12))
	if got := kicked(e); got != first {
		t.Errorf("build painted %q, want %q again", got, first)
	}
}

func kicked(e *Engine) string {
	for l, lt := range e.lights {
		q := lt.queue
		for _, m := range q {
			if m.Drop && m.Payload["color"] != nil {
				return l
			}
		}
	}
	return ""
}

// TestAClearedKickIsPaintedAgain.
func TestAClearedKickIsPaintedAgain(t *testing.T) {
	for name, clearIt := range map[string]func(e *Engine, light string){
		"paused":       func(e *Engine, _ string) { e.paused(t0, "test") },
		"disconnected": func(e *Engine, _ string) { e.Disconnected() },
		"left alone": func(e *Engine, l string) {
			e.lights[l].fails = awayAfter - 1
			e.failed(t0, l)
			e.lights[l].away = false
		},
	} {
		e := started(t, "separate", 100)
		eff := testConfig("separate").Effects
		eff.Variant = "build"
		e.SetEffects(t0, eff)
		e.Music(t0, moment(8))
		first := kicked(e)
		clearIt(e, first)
		e.bo = flow.Backoff{}
		e.Music(t0, moment(12))
		if got := kicked(e); got != first {
			t.Errorf("%s: build painted %q, want %q again", name, got, first)
		}
	}
}
