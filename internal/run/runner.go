package run

import (
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run/engine"
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
	eng      *engine.Engine
	hooks    Hooks
	offset   time.Duration // the audio offset now, also across engine restarts
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

// report hands the stats to the Status hook, if there is one.
func (r *runner) report(st engine.Stats) {
	if r.hooks.Status != nil {
		r.hooks.Status(st)
	}
}
