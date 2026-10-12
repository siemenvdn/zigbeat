package run

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
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

// TestStyleOutlivesTheEngine: the loop keeps a new style in the config, from
// which the engine after a panic is built.
func TestStyleOutlivesTheEngine(t *testing.T) {
	r, _ := newShell(t)
	r.eng = engine.New(r.cfg, 0, time.Now())
	styles := make(chan config.Effects, 1)
	r.hooks.Effects = styles
	eff := r.cfg.Effects
	eff.Variant = "swap"
	styles <- eff
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.play(ctx, music.Metronome{BPM: 120, Lookahead: time.Second}); close(done) }()
	for deadline := time.Now().Add(time.Second); len(styles) > 0; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the loop did not take the style")
		}
	}
	cancel()
	<-done
	if r.cfg.Effects.Variant != "swap" {
		t.Errorf("variant %q, want swap", r.cfg.Effects.Variant)
	}
}
