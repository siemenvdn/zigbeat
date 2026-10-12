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
	tickEvery   = 10 * time.Millisecond // how late a message may leave: far below the engine's late limit
	statusEvery = 10 * time.Second      // a status line often enough to follow, seldom enough to read
)

// play runs the show until ctx is done or the source ends. Network trouble never
// ends it: backoff pauses, a lost connection is dialled again. A panic returns
// with panicked set, so Run can start a fresh engine.
func (r *runner) play(ctx context.Context, src music.Source) (reason string, panicked bool) {
	defer func() {
		if p := recover(); p != nil {
			reason, panicked = fmt.Sprintf("panic: %v\n%s", p, debug.Stack()), true
		}
	}()
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
		case <-r.link.Lost():
			fmt.Printf("%s  mqtt connection lost (%v); reconnecting every %s\n", cli.Stamp(), r.link.Err(), cli.Seconds(cli.RedialEvery))
			r.eng.Disconnected()
			if !r.link.Reconnect(ctx) {
				return "interrupted", false
			}
			fmt.Printf("%s  reconnected\n", cli.Stamp())
		case ev, ok := <-events:
			if !ok {
				return "the beat source ended", false
			}
			r.eng.Music(time.Now(), ev)
		case ev := <-r.events:
			r.handle(ev)
		case <-tick.C:
			// time.Now, not the tick: after a reconnect a ticker holds a stale one.
			for _, s := range r.eng.Due(time.Now()) {
				r.publish(s)
			}
		case <-status.C:
			st := r.eng.Stats(time.Now())
			fmt.Println(st.Line())
			r.report(st)
		case d := <-r.hooks.Offsets:
			r.offset = d
			r.eng.SetOffset(d)
			fmt.Printf("%s  audio offset %d ms\n", cli.Stamp(), d.Milliseconds())
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

// drain halts the engine and lets the messages in flight be confirmed, for at most d.
// The engine may have just panicked; a second panic only ends the wait.
func (r *runner) drain(d time.Duration) {
	defer func() {
		if p := recover(); p != nil {
			fmt.Fprintf(os.Stderr, "%s  panic while stopping: %v\n", cli.Stamp(), p)
		}
	}()
	r.eng.Halt()
	if r.offline() { // nothing in flight can be confirmed any more
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
		case <-r.link.Lost():
			return
		case <-deadline:
			return
		}
	}
}
