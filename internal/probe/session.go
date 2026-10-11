package probe

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/siemenvdn/zigbeat/internal/mqtt"
	"github.com/siemenvdn/zigbeat/internal/probe/measure"
	"github.com/siemenvdn/zigbeat/internal/probe/palette"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

type stateAt struct {
	s  z2m.State
	at time.Time
}

// session holds what the phases of one probe run share.
type session struct {
	cfg            config
	ctx            context.Context
	client         *mqtt.Client
	bridge         *z2m.Bridge
	rec            *measure.Recorder
	ph             *measure.Phase
	gen            *palette.Generator
	tracker        *measure.Tracker
	canary         *measure.Canary
	canaryAnswered chan struct{}

	mu       sync.Mutex
	last     map[string]stateAt // latest state echo per target
	snaps    map[string]z2m.State
	snapJSON []byte

	stopBg            func()
	intOnce, lostOnce sync.Once
}

// connect dials the broker and subscribes to the targets, the canary and the Zigbee2MQTT log.
func connect(ctx context.Context, cfg config) (*session, error) {
	client, err := mqtt.Dial(cfg.broker, mqtt.Options{
		ClientID:  fmt.Sprintf("zigbeat-probe-%d", os.Getpid()),
		Username:  cfg.user,
		Password:  cfg.password,
		KeepAlive: 20 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("mqtt: %v", err)
	}
	rec := &measure.Recorder{}
	s := &session{
		cfg:            cfg,
		ctx:            ctx,
		client:         client,
		bridge:         z2m.New(client, cfg.base),
		rec:            rec,
		ph:             measure.NewPhase("setup"),
		gen:            palette.New(cfg.mode),
		tracker:        measure.NewTracker(rec, cfg.timeout, palette.KeyFunc(cfg.mode)),
		canary:         measure.NewCanary(cfg.canary, rec, cfg.timeout),
		canaryAnswered: make(chan struct{}, 1),
		last:           map[string]stateAt{},
		snaps:          map[string]z2m.State{},
		stopBg:         func() {},
	}
	if err := s.subscribe(); err != nil {
		client.Close()
		return nil, fmt.Errorf("subscribe: %v", err)
	}
	return s, nil
}

func (s *session) subscribe() error {
	for _, t := range s.cfg.targets {
		err := s.bridge.OnState(t, func(st z2m.State, at time.Time) {
			s.mu.Lock()
			s.last[t] = stateAt{st, at}
			s.mu.Unlock()
			s.tracker.OnState(t, st, at)
		})
		if err != nil {
			return err
		}
	}
	err := s.bridge.OnState(s.cfg.canary, func(_ z2m.State, at time.Time) {
		s.canary.OnState(at)
		select {
		case s.canaryAnswered <- struct{}{}:
		default:
		}
	})
	if err != nil {
		return err
	}
	return s.bridge.OnLog(func(e z2m.LogEntry, at time.Time) {
		if e.Level == "error" || e.Level == "warning" {
			s.rec.AddLog(measure.LogEvent{At: at, Level: e.Level, Msg: e.Message})
		}
	})
}

// answeredSince returns the latest state of dev if it arrived after since.
func (s *session) answeredSince(dev string, since time.Time) (z2m.State, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.last[dev]
	return v.s, ok && v.at.After(since)
}
