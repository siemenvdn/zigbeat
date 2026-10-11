package run

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/run/engine"
)

// play runs the show until ctx is done or the source ends. Network trouble never
// ends it: backoff pauses, a lost connection is dialled again. A panic returns
// with panicked set, so Run can start a fresh engine.
func (r *runner) play(ctx context.Context, src beat.Source) (reason string, panicked bool) {
	defer func() {
		if p := recover(); p != nil {
			reason, panicked = fmt.Sprintf("panic: %v\n%s", p, debug.Stack()), true
		}
	}()
	srcCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	beats := src.Beats(srcCtx)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	status := time.NewTicker(10 * time.Second)
	defer status.Stop()
	r.report(r.eng.Stats(time.Now()))
	for {
		select {
		case <-ctx.Done():
			return "interrupted", false
		case <-r.link.Lost():
			fmt.Printf("%s  mqtt connection lost (%v); reconnecting every 5 s\n", cli.Stamp(), r.link.Err())
			r.eng.Disconnected()
			if !r.link.Reconnect(ctx) {
				return "interrupted", false
			}
			fmt.Printf("%s  reconnected\n", cli.Stamp())
		case b, ok := <-beats:
			if !ok {
				return "the beat source ended", false
			}
			r.eng.Beat(time.Now(), b)
		case ev := <-r.events:
			r.handle(ev)
		case now := <-tick.C:
			for _, s := range r.eng.Due(now) {
				r.publish(s)
			}
		case now := <-status.C:
			st := r.eng.Stats(now)
			fmt.Println(st.Line())
			r.report(st)
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
	deadline := time.After(d)
	tick := time.NewTicker(10 * time.Millisecond)
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
