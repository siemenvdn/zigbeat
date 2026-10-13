package probe

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/lights"
	"github.com/siemenvdn/zigbeat/internal/probe/measure"
	"github.com/siemenvdn/zigbeat/internal/probe/palette"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// session holds what the phases of one probe run share.
type session struct {
	*lights.Session
	cfg     options
	ctx     context.Context
	rec     *measure.Recorder
	ph      *measure.Phase
	gen     *palette.Generator
	tracker *measure.Tracker
	canary  *measure.Canary

	snaps    map[string]z2m.State
	snapJSON []byte

	stopBg            func()
	intOnce, lostOnce sync.Once
}

// connect dials the broker and subscribes to the targets, the canary and the Zigbee2MQTT log.
func connect(ctx context.Context, cfg options) (*session, error) {
	rec := &measure.Recorder{}
	s := &session{
		cfg:     cfg,
		ctx:     ctx,
		rec:     rec,
		ph:      measure.NewPhase("setup"),
		gen:     palette.New(cfg.mode),
		tracker: measure.NewTracker(rec, cfg.timeout, palette.KeyFunc(cfg.mode)),
		canary:  measure.NewCanary(cfg.canary, rec, cfg.timeout),
		snaps:   map[string]z2m.State{},
		stopBg:  func() {},
	}
	m := config.MQTT{Broker: cfg.broker, Username: cfg.user, Password: cfg.password, BaseTopic: cfg.base}
	zb, err := lights.Dial(m, "probe", lights.Watch{
		Devices: append(slices.Clone(cfg.targets), cfg.canary),
		OnState: func(d string, st z2m.State, at time.Time) {
			if d == cfg.canary {
				s.canary.OnState(at)
				return
			}
			s.tracker.OnState(d, st, at)
		},
		OnLog: func(e z2m.LogEntry, at time.Time) {
			if e.Level == "error" || e.Level == "warning" {
				rec.AddLog(measure.LogEvent{At: at, Level: e.Level, Msg: e.Message})
			}
		},
	})
	if err != nil {
		return nil, err
	}
	s.Session = zb
	return s, nil
}
