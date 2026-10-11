package run

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// event is a state message or an error line from Zigbee2MQTT.
type event struct {
	light string // empty for a log line
	st    z2m.State
	msg   string
	at    time.Time
}

// runner connects the engine to Zigbee2MQTT and real time.
type runner struct {
	cfg      config.Config
	link     link
	events   chan event
	eng      *engine
	snaps    map[string]z2m.State
	snapJSON []byte
}

func connect(cfg config.Config) (*runner, error) {
	r := &runner{cfg: cfg, events: make(chan event, 1024)}
	l, err := dialLink(cfg.MQTT, r.watched, r.push)
	if err != nil {
		return nil, err
	}
	r.link = l
	return r, nil
}

// watched lists the devices to subscribe to: the lights still in the show and the canary.
func (r *runner) watched() []string {
	ds := r.cfg.Lights()
	if r.cfg.Canary != "" {
		ds = append(ds, r.cfg.Canary)
	}
	return ds
}

// push hands an event to the loop without blocking the MQTT reader.
func (r *runner) push(ev event) {
	select {
	case r.events <- ev:
	default:
	}
}

func (r *runner) offline() bool {
	select {
	case <-r.link.Lost():
		return true
	default:
		return false
	}
}

func stamp() string { return time.Now().Format(cli.Clock) }

// play runs the show until ctx is done or the source ends. Network trouble never
// ends it: backoff pauses, a lost connection is dialled again. A panic returns
// with panicked set, so Run can start a fresh engine.
func (r *runner) play(ctx context.Context, src beat.Source) (reason string, panicked bool) {
	defer func() {
		if p := recover(); p != nil {
			reason, panicked = fmt.Sprintf("panic: %v", p), true
		}
	}()
	srcCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	beats := src.Beats(srcCtx)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	status := time.NewTicker(10 * time.Second)
	defer status.Stop()
	for {
		select {
		case <-ctx.Done():
			return "interrupted", false
		case <-r.link.Lost():
			fmt.Printf("%s  mqtt connection lost (%v); reconnecting every 5 s\n", stamp(), r.link.Err())
			r.eng.disconnected()
			if !r.link.Reconnect(ctx) {
				return "interrupted", false
			}
			fmt.Printf("%s  reconnected\n", stamp())
		case b, ok := <-beats:
			if !ok {
				return "the beat source ended", false
			}
			r.eng.beat(time.Now(), b)
		case ev := <-r.events:
			r.handle(ev)
		case now := <-tick.C:
			for _, s := range r.eng.due(now) {
				r.publish(s)
			}
		case now := <-status.C:
			fmt.Println(r.eng.status(now))
		}
		for _, n := range r.eng.takeNotes() {
			fmt.Println(n)
		}
	}
}

func (r *runner) handle(ev event) {
	if ev.light == "" {
		r.eng.zigbeeError(ev.at, ev.msg)
		return
	}
	r.eng.state(ev.light, ev.at)
}

func (r *runner) publish(s send) {
	if s.get {
		_ = r.link.Get(s.light, s.payload)
		return
	}
	_ = r.link.Set(s.light, s.payload)
}

// drain halts the engine and lets the messages in flight be confirmed, for at most d.
// The engine may have just panicked; a second panic only ends the wait.
func (r *runner) drain(d time.Duration) {
	defer func() {
		if p := recover(); p != nil {
			fmt.Fprintf(os.Stderr, "%s  panic while stopping: %v\n", stamp(), p)
		}
	}()
	r.eng.halt()
	deadline := time.After(d)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for r.eng.inFlight() > 0 {
		select {
		case ev := <-r.events:
			r.handle(ev)
		case now := <-tick.C:
			r.eng.due(now)
		case <-r.link.Lost():
			return
		case <-deadline:
			return
		}
	}
}
