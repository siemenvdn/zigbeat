package run

import (
	"context"
	"testing"
	"time"

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

func (f *fakeLink) Lost() <-chan struct{} { return f.lost }

func (f *fakeLink) Err() error { return nil }

func (f *fakeLink) Close() {}

func (f *fakeLink) Groups(context.Context) ([]string, bool) { return f.groups, f.groups != nil }

func (f *fakeLink) Reconnect(context.Context) bool {
	f.reconnects++
	if f.reconnect {
		f.lost = make(chan struct{})
	}
	return f.reconnect
}
