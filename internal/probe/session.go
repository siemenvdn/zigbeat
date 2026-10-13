package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/lights"
	"github.com/siemenvdn/zigbeat/internal/probe/palette"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

type session struct {
	*lights.Session
	cfg     options
	ctx     context.Context
	rec     *Recorder
	ph      *Phase
	gen     *palette.Generator
	tracker *Tracker
	canary  *Canary

	snaps    map[string]z2m.State
	snapJSON []byte

	stopBg            func()
	intOnce, lostOnce sync.Once
}

func connect(ctx context.Context, cfg options) (*session, error) {
	rec := &Recorder{}
	s := &session{
		cfg:     cfg,
		ctx:     ctx,
		rec:     rec,
		ph:      NewPhase("setup"),
		gen:     palette.New(cfg.mode),
		tracker: NewTracker(rec, cfg.timeout, palette.KeyFunc(cfg.mode)),
		canary:  NewCanary(cfg.canary, rec, cfg.timeout),
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
				rec.AddLog(LogEvent{At: at, Level: e.Level, Msg: e.Message})
			}
		},
	})
	if err != nil {
		return nil, err
	}
	s.Session = zb
	return s, nil
}

func (s *session) saveState() bool {
	s.snaps = lights.Snapshot(s.ctx, s, s.cfg.targets, s.cfg.rates[0])
	if s.ctx.Err() != nil {
		fmt.Println("cancelled, nothing was changed")
		return false
	}
	ok := true
	for _, t := range s.cfg.targets {
		if _, got := s.snaps[t]; !got {
			fmt.Fprintf(os.Stderr, "no answer from %s; check the friendly name and that it is powered\n", t)
			ok = false
		}
	}
	s.snapJSON, _ = json.Marshal(s.snaps)
	return ok
}

func (s *session) restore() {
	s.ph.Set("restore", 0)
	fmt.Println()
	lights.Restore(s, s.snaps, s.cfg.targets, s.cfg.rates[0])
}

// Online lets the restore stop when the broker is gone, saying once what to restore by hand.
func (s *session) Online() bool { return !s.connLost() }
