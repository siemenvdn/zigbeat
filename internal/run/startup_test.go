package run

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestStartupStopsOnCtrlC(t *testing.T) {
	r, f := newShell(t)
	f.silent = true
	r.cfg.Canary = "far-lamp"
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	if r.snapshot(ctx) {
		t.Fatal("nothing answered")
	}
	r.canaryBaseline(ctx)
	if d := time.Since(start); d > time.Second {
		t.Fatalf("startup took %s after Ctrl-C", d)
	}
}

func TestStartupRefusesGroups(t *testing.T) {
	r, f := newShell(t)
	if err := r.refuseGroups(context.Background()); err != nil {
		t.Fatalf("no groups known: %v", err)
	}
	f.groups = []string{"living-room", "lamp-2"}
	if err := r.refuseGroups(context.Background()); err == nil || !strings.Contains(err.Error(), "lamp-2") {
		t.Fatalf("lamp-2 is a group: %v", err)
	}
}
