package run

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/lights"
	"github.com/siemenvdn/zigbeat/internal/run/engine"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

type event struct {
	light string
	st    z2m.State
	msg   string
	at    time.Time
}

type runner struct {
	cfg      config.Config
	link     link
	events   chan event
	eng      *engine.Engine
	hooks    Hooks
	snaps    map[string]z2m.State
	snapJSON []byte
}

const eventBuffer = 1024

func connect(ctx context.Context, cfg config.Config) (*runner, error) {
	r := &runner{cfg: cfg, events: make(chan event, eventBuffer)}
	type dialed struct {
		l   *lights.Session
		err error
	}
	done := make(chan dialed, 1)
	go func() {
		l, err := dialLink(cfg, r.push)
		done <- dialed{l, err}
	}()
	select {
	case d := <-done:
		if d.err != nil {
			return nil, d.err
		}
		r.link = d.l
		return r, nil
	case <-ctx.Done():
		go func() {
			if d := <-done; d.err == nil {
				d.l.Close()
			}
		}()
		return nil, ctx.Err()
	}
}

func (r *runner) push(ev event) {
	select {
	case r.events <- ev:
	default:
	}
}

func (r *runner) offline() bool {
	select {
	case <-r.link.Done():
		return true
	default:
		return false
	}
}

func (r *runner) report(st engine.Stats) {
	if r.hooks.Status != nil {
		r.hooks.Status(st)
	}
}
