package run

import (
	"context"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

type fakeLink struct {
	events     chan event
	silent     bool
	lost       chan struct{}
	dropAt     int
	reconnect  bool
	sets       int
	reconnects int
	groups     []string
	devices    []z2m.Device
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

func (f *fakeLink) Done() <-chan struct{} { return f.lost }

// Await answers at once, unless the fake is silent or the connection dropped.
func (f *fakeLink) Await(context.Context, string, time.Time, time.Duration) (z2m.State, bool) {
	select {
	case <-f.lost:
		return z2m.State{}, false
	default:
	}
	return z2m.State{State: "ON"}, !f.silent
}

func (f *fakeLink) Read(context.Context, string, time.Duration) (time.Duration, bool) {
	return time.Millisecond, !f.silent
}

func (f *fakeLink) Err() error { return nil }

func (f *fakeLink) Close() {}

func (f *fakeLink) Groups(context.Context) ([]string, error) {
	if f.groups == nil {
		return nil, z2m.ErrNoRetained
	}
	return f.groups, nil
}

func (f *fakeLink) Devices(context.Context) ([]z2m.Device, error) {
	if f.devices == nil {
		return nil, z2m.ErrNoRetained
	}
	return f.devices, nil
}

func (f *fakeLink) Reconnect(context.Context) bool {
	f.reconnects++
	if f.reconnect {
		f.lost = make(chan struct{})
	}
	return f.reconnect
}
