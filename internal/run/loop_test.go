package run

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/engine"
)

func TestDrainSurvivesAPanickedEngine(t *testing.T) {
	r, _ := newShell(t)
	r.eng = &engine.Engine{} // broken: any call into it panics
	r.drain(50 * time.Millisecond)
}

func TestPanicReasonCarriesTheStack(t *testing.T) {
	r, _ := newShell(t)
	r.eng = &engine.Engine{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	why, panicked := r.play(ctx, music.Metronome{BPM: 120, Lookahead: time.Second})
	if !panicked || !strings.Contains(why, "goroutine ") {
		t.Errorf("panicked %v, reason %q", panicked, why)
	}
}

// panicSource panics each time a show starts it.
type panicSource struct{ starts *int }

func (s panicSource) Events(context.Context) <-chan music.Event {
	*s.starts++
	panic("test")
}

func TestPanicEndsTheShow(t *testing.T) {
	r, _ := newShell(t)
	starts := 0
	why, code := r.show(context.Background(), panicSource{&starts}, 0)
	if code != 1 || starts != 1 || !strings.Contains(why, "bug") {
		t.Errorf("code %d after %d starts, reason %q; want 1 after one start", code, starts, why)
	}
}
