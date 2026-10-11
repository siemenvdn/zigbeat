package run

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/run/engine"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// fakeLink answers every command with a state, can drop the connection at a
// given Set, and reconnects or not.
type fakeLink struct {
	events     chan event
	silent     bool // never answer
	lost       chan struct{}
	dropAt     int // close lost at this Set instead of answering; 0 never
	reconnect  bool
	sets       int
	reconnects int
	groups     []string // nil: bridge/groups never arrives
}

func newShell(t *testing.T) (*runner, *fakeLink) {
	t.Helper()
	cfg := testConfig("separate")
	cfg.Budget.MaxRate = 1000
	r := &runner{cfg: cfg, events: make(chan event, 1024), snaps: map[string]z2m.State{}}
	for _, l := range cfg.Lights() {
		r.snaps[l] = z2m.State{State: "ON"}
	}
	f := &fakeLink{events: r.events, lost: make(chan struct{})}
	r.link = f
	return r, f
}

func (f *fakeLink) answer(light string) {
	if !f.silent {
		f.events <- event{light: light, at: time.Now().Add(time.Millisecond)}
	}
}

func (f *fakeLink) Set(light string, _ any) error {
	f.sets++
	if f.sets == f.dropAt {
		close(f.lost)
		return nil
	}
	f.answer(light)
	return nil
}

func (f *fakeLink) Get(light string, _ any) error { f.answer(light); return nil }
func (f *fakeLink) Lost() <-chan struct{}         { return f.lost }
func (f *fakeLink) Err() error                    { return nil }
func (f *fakeLink) Close()                        {}

func (f *fakeLink) Groups(context.Context) ([]string, bool) { return f.groups, f.groups != nil }

func (f *fakeLink) Reconnect(context.Context) bool {
	f.reconnects++
	if f.reconnect {
		f.lost = make(chan struct{})
	}
	return f.reconnect
}

func TestRestoreSurvivesAReconnect(t *testing.T) {
	r, f := newShell(t)
	f.dropAt, f.reconnect = 2, true
	if !r.restore() {
		t.Fatal("the broker came back, so the restore must finish")
	}
	if f.reconnects != 1 || f.sets != 5 {
		t.Errorf("reconnects %d, sets %d: want 1 reconnect and lamp-2 sent again", f.reconnects, f.sets)
	}
}

func TestRestoreReportsAGoneBroker(t *testing.T) {
	r, f := newShell(t)
	f.dropAt = 2
	if r.restore() {
		t.Fatal("the broker stayed away, so the restore must fail")
	}
	if f.reconnects != 1 || f.sets != 2 {
		t.Errorf("reconnects %d, sets %d", f.reconnects, f.sets)
	}
}

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

func TestDrainSurvivesAPanickedEngine(t *testing.T) {
	r, _ := newShell(t)
	r.eng = &engine.Engine{} // broken: any call into it panics
	r.drain(50 * time.Millisecond)
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
