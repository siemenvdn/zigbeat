package run

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/mqtt"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// event is a state message or an error line from Zigbee2MQTT.
type event struct {
	light string // empty for a log line
	st    z2m.State
	msg   string
	at    time.Time
}

// runner connects the engine to MQTT and real time.
type runner struct {
	cfg      config.Config
	client   *mqtt.Client
	bridge   *z2m.Bridge
	events   chan event
	eng      *engine
	snaps    map[string]z2m.State
	snapJSON []byte
}

func connect(cfg config.Config) (*runner, error) {
	r := &runner{cfg: cfg, events: make(chan event, 1024)}
	return r, r.dial()
}

// dial connects to the broker and subscribes to the lights, the canary and the log.
func (r *runner) dial() error {
	client, err := mqtt.Dial(r.cfg.MQTT.Broker, mqtt.Options{
		ClientID:  fmt.Sprintf("zigbeat-run-%d", os.Getpid()),
		Username:  r.cfg.MQTT.Username,
		Password:  r.cfg.MQTT.Password,
		KeepAlive: 20 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("mqtt: %v", err)
	}
	bridge := z2m.New(client, r.cfg.MQTT.BaseTopic)
	watch := r.cfg.Lights()
	if r.cfg.Canary != "" {
		watch = append(watch, r.cfg.Canary)
	}
	for _, l := range watch {
		if err := bridge.OnState(l, func(st z2m.State, at time.Time) { r.push(event{light: l, st: st, at: at}) }); err != nil {
			client.Close()
			return fmt.Errorf("subscribe: %v", err)
		}
	}
	err = bridge.OnLog(func(e z2m.LogEntry, at time.Time) {
		if e.Level == "error" {
			r.push(event{msg: e.Message, at: at})
		}
	})
	if err != nil {
		client.Close()
		return fmt.Errorf("subscribe: %v", err)
	}
	r.client, r.bridge = client, bridge
	return nil
}

// redial connects again every 5 s until it works or ctx is done.
func (r *runner) redial(ctx context.Context) bool {
	r.client.Close()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(5 * time.Second):
		}
		err := r.dial()
		if err == nil {
			return true
		}
		fmt.Fprintf(os.Stderr, "reconnect: %v\n", err)
	}
}

// push hands an event to the loop without blocking the MQTT reader.
func (r *runner) push(ev event) {
	select {
	case r.events <- ev:
	default:
	}
}

func (r *runner) connLost() bool {
	select {
	case <-r.client.Done():
		return true
	default:
		return false
	}
}

func stamp() string { return time.Now().Format("15:04:05") }

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
		case <-r.client.Done():
			fmt.Printf("%s  mqtt connection lost (%v); reconnecting every 5 s\n", stamp(), r.client.Err())
			r.eng.disconnected()
			if !r.redial(ctx) {
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
		_ = r.bridge.Get(s.light, s.payload)
		return
	}
	_ = r.bridge.Set(s.light, s.payload)
}

// drain lets the messages in flight be confirmed, for at most d.
func (r *runner) drain(d time.Duration) {
	deadline := time.After(d)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for r.eng.inFlight() > 0 {
		select {
		case ev := <-r.events:
			r.handle(ev)
		case now := <-tick.C:
			r.eng.due(now)
		case <-deadline:
			return
		}
	}
}
