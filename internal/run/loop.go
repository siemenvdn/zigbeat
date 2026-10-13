package run

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/engine"
)

const (
	tickEvery   = 10 * time.Millisecond
	statusEvery = 10 * time.Second
)

func (r *runner) play(ctx context.Context, src music.Source) (reason string, panicked bool) {
	defer func() {
		if p := recover(); p != nil {
			reason, panicked = fmt.Sprintf("panic: %v\n%s", p, debug.Stack()), true
		}
	}()
	for len(r.events) > 0 {
		<-r.events
	}
	srcCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	events := src.Events(srcCtx)
	tick := time.NewTicker(tickEvery)
	defer tick.Stop()
	status := time.NewTicker(statusEvery)
	defer status.Stop()
	r.report(r.eng.Stats(time.Now()))
	for {
		select {
		case <-ctx.Done():
			return "interrupted", false
		case <-r.link.Done():
			cli.Say("mqtt connection lost (%v); reconnecting every %s", r.link.Err(), cli.Seconds(cli.RedialEvery))
			r.eng.Disconnected()
			if !r.link.Reconnect(ctx) {
				return "interrupted", false
			}
			cli.Say("reconnected")
		case ev, ok := <-events:
			if !ok {
				return "the beat source ended", false
			}
			r.eng.Music(time.Now(), ev)
		case ev := <-r.events:
			r.handle(ev)
		case <-tick.C:

			for _, s := range r.eng.Due(time.Now()) {
				r.publish(s)
			}
		case <-status.C:
			st := r.eng.Stats(time.Now())
			fmt.Println(st.Line())
			r.report(st)
		case d := <-r.hooks.Offsets:
			r.eng.SetOffset(d)
			cli.Say("audio offset %d ms", d.Milliseconds())
		case eff := <-r.hooks.Effects:
			r.eng.SetEffects(time.Now(), eff)
		}
		for _, n := range r.eng.TakeNotes() {
			fmt.Println(n)
		}
	}
}

func (r *runner) handle(ev event) {
	if ev.light == "" {
		r.eng.ZigbeeError(ev.at, ev.msg)
		return
	}
	r.eng.State(ev.light, ev.at)
}

func (r *runner) publish(s engine.Send) {
	if s.Get {
		_ = r.link.Get(s.Light, s.Payload)
		return
	}
	_ = r.link.Set(s.Light, s.Payload)
}

func (r *runner) drain(d time.Duration) {
	defer func() {
		if p := recover(); p != nil {
			fmt.Fprintf(os.Stderr, "%s  panic while stopping: %v\n", cli.Stamp(), p)
		}
	}()
	r.eng.Halt()
	if r.offline() {
		return
	}
	deadline := time.After(d)
	tick := time.NewTicker(tickEvery)
	defer tick.Stop()
	for r.eng.InFlight() > 0 {
		select {
		case ev := <-r.events:
			r.handle(ev)
		case now := <-tick.C:
			r.eng.Due(now)
		case <-r.link.Done():
			return
		case <-deadline:
			return
		}
	}
}
