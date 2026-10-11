package probe

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/siemenvdn/zigbeat/internal/mqtt"
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
	rec            *recorder
	ph             *phaseState
	gen            *generator
	tracker        *cmdTracker
	canary         *canaryProbe
	canaryAnswered chan struct{}

	mu       sync.Mutex
	last     map[string]stateAt // latest state echo per target
	snaps    map[string]z2m.State
	snapJSON []byte

	stopSignals, stopBg func()
	intOnce, lostOnce   sync.Once
}

// connect dials the broker and subscribes to the targets, the canary and the Zigbee2MQTT log.
func connect(ctx context.Context, cfg config, stopSignals func()) (*session, error) {
	client, err := mqtt.Dial(cfg.broker, mqtt.Options{
		ClientID:  fmt.Sprintf("zigbeat-probe-%d", os.Getpid()),
		Username:  cfg.user,
		Password:  cfg.password,
		KeepAlive: 20 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("mqtt: %v", err)
	}
	rec := &recorder{}
	s := &session{
		cfg:            cfg,
		ctx:            ctx,
		client:         client,
		bridge:         z2m.New(client, cfg.base),
		rec:            rec,
		ph:             &phaseState{name: "setup"},
		gen:            newGenerator(cfg.mode),
		tracker:        newCmdTracker(rec, cfg.timeout, keyFunc(cfg.mode)),
		canary:         &canaryProbe{device: cfg.canary, rec: rec, timeout: cfg.timeout},
		canaryAnswered: make(chan struct{}, 1),
		last:           map[string]stateAt{},
		snaps:          map[string]z2m.State{},
		stopSignals:    stopSignals,
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
			s.tracker.onState(t, st, at)
		})
		if err != nil {
			return err
		}
	}
	err := s.bridge.OnState(s.cfg.canary, func(_ z2m.State, at time.Time) {
		s.canary.onState(at)
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
			s.rec.addLog(logEvent{at: at, level: e.Level, msg: e.Message})
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

// startBackground starts the canary probes and the expiry of unanswered commands.
func (s *session) startBackground() {
	bg, stop := context.WithCancel(context.Background())
	s.stopBg = stop
	// Irregular probe interval so probes never lock in phase with the command rate.
	go func() {
		for {
			d := 700*time.Millisecond + time.Duration(rand.Int63n(int64(600*time.Millisecond)))
			select {
			case <-bg.Done():
				return
			case now := <-time.After(d):
				s.canary.tick(now, s.ph, func() error { return s.bridge.Get(s.cfg.canary, map[string]string{"state": ""}) })
			}
		}
	}()
	go every(bg, 200*time.Millisecond, s.tracker.expire)
}

func (s *session) interrupted() bool {
	if s.ctx.Err() == nil {
		return false
	}
	s.intOnce.Do(func() {
		s.stopSignals()
		fmt.Println("\ninterrupted (Ctrl-C again to quit without restoring)")
	})
	return true
}

func (s *session) connLost() bool {
	if !s.client.Closed() {
		return false
	}
	s.lostOnce.Do(func() {
		fmt.Fprintf(os.Stderr, "\nmqtt connection lost: %v\nrestore by hand from: %s\n", s.client.Err(), s.snapJSON)
	})
	return true
}

// finish stops the background work, prints the summary and writes the raw samples.
func (s *session) finish(results []stepResult, base latencyStats) {
	s.stopBg()
	if len(results) > 0 {
		printSummary(os.Stdout, results, base, s.cfg.canary)
	}
	samples, logs := s.rec.snapshot()
	if err := writeCSV(s.cfg.out, samples, logs); err != nil {
		fmt.Fprintf(os.Stderr, "csv: %v\n", err)
	} else {
		fmt.Printf("raw samples: %s\n", s.cfg.out)
	}
}
