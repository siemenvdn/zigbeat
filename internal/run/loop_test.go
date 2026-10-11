package run

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
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
	why, panicked := r.play(ctx, beat.Metronome{BPM: 120, Lookahead: time.Second})
	if !panicked || !strings.Contains(why, "goroutine ") {
		t.Errorf("panicked %v, reason %q", panicked, why)
	}
}
