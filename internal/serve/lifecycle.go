package serve

import (
	"context"
	"sync"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run"
	"github.com/siemenvdn/zigbeat/internal/run/engine"
	"github.com/siemenvdn/zigbeat/internal/serve/ha"
)

type server struct {
	show    func(ctx context.Context, h run.Hooks) int
	cmds    chan bool
	styles  <-chan config.Effects
	offsets <-chan time.Duration
	put     func(ha.State)
	total   int
}

func newest[T any](ch chan T, v T) {
	select {
	case <-ch:
	default:
	}
	ch <- v
}

func (s *server) loop(ctx context.Context) {
	for {
		s.put(ha.Off)
		cli.Say("waiting for the switch in Home Assistant")
		if !s.waitOn(ctx) {
			return
		}
		s.play(ctx)
	}
}

func (s *server) waitOn(ctx context.Context) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		case on := <-s.cmds:
			if on {
				return true
			}
		}
	}
}

func (s *server) play(ctx context.Context) {
	cli.Say("switched on in Home Assistant")
	s.put(ha.Starting)
	showCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	st := &states{put: s.put}
	stopping, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		s.show(showCtx, run.Hooks{
			Effects: s.styles,
			Offsets: s.offsets,
			Status:  func(stats engine.Stats) { st.playing(ha.Playing(stats, s.total)) },
			Stopping: func() string {
				stopping <- struct{}{}
				st.stopping()
				if ctx.Err() != nil {
					return cli.QuitHint
				}
				return ""
			},
		})
	}()
	cmds := s.cmds
	for {
		select {
		case on := <-cmds:
			if !on {
				cli.Say("switched off in Home Assistant")
				st.stopping()
				cancel()
				cmds = nil
			}
		case <-stopping:
			cmds = nil
		case <-done:
			return
		}
	}
}

type states struct {
	mu   sync.Mutex
	put  func(ha.State)
	stop bool
}

func (t *states) playing(st ha.State) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.stop {
		t.put(st)
	}
}

func (t *states) stopping() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stop = true
	t.put(ha.Stopping)
}
