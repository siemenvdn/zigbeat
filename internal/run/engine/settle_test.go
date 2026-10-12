package engine

import "testing"

// gap plays one moment and its next beat, then lets the music stop.
func gap(e *Engine) {
	e.Beat(at(1000), moment(4))
	e.Beat(at(1500), moment(5))
}

// isSettle is the low end for a breathe light that a gap sends: a brightness without a move.
func isSettle(p map[string]any) bool { return p["brightness"] != nil && p["brightness_move"] == nil }

func TestGapSettlesBreathe(t *testing.T) {
	e := started(t, "separate", 100)
	gap(e)
	settledAt := 0
	for ms := 1000; ms < 6000; ms += 10 {
		for _, s := range e.Due(at(ms)) {
			if s.Light == "lamp-4" && isSettle(s.Payload) {
				settledAt = ms
			}
			e.State(s.Light, at(ms+20))
		}
	}
	if settledAt < 3000 {
		t.Errorf("lamp-4 gets its low end a beat after the latest beat (3 s), got %d ms", settledAt)
	}
}

func TestNoSettleWhilePausedOrStopping(t *testing.T) {
	for name, stop := range map[string]func(*Engine){
		"paused":   func(e *Engine) { e.bo.Pause(at(1000)) },
		"stopping": func(e *Engine) { e.Halt() },
	} {
		e := started(t, "separate", 100)
		gap(e)
		stop(e)
		e.Due(at(5000))
		for _, q := range e.queue["lamp-4"] {
			if isSettle(q.Payload) {
				t.Errorf("%s: settle queued: %+v", name, q)
			}
		}
	}
}
