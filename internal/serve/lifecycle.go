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

// server switches the show on and off for Home Assistant.
type server struct {
	show    func(ctx context.Context, h run.Hooks) int // run.Play in production
	cmds    chan bool                                  // switch commands, ON is true; newest wins
	styles  <-chan config.Effects                      // the style after a choice in Home Assistant
	offsets <-chan time.Duration                       // the audio offset after a choice in Home Assistant
	put     func(ha.State)                             // hands a state to the link; never blocks
	total   int                                        // lights in the config
}

// newest puts v in a channel of one, replacing what is waiting there.
// Only one goroutine may send on ch.
func newest[T any](ch chan T, v T) {
	select {
	case <-ch:
	default:
	}
	ch <- v
}

// loop serves until ctx is done, which only a signal does. A running show is
// stopped and restored before it returns.
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

// waitOn waits for ON and reports false when ctx is done first.
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

// play runs one show. OFF cancels it at once, also during the start, and ON is
// ignored while it runs. From OFF or the restore on, commands wait in cmds for the loop.
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
				stopping <- struct{}{} // returns once play no longer reads cmds
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

// states passes a show's states on to Home Assistant. Once it is stopping, a
// running state that the show loop sent late is held back.
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
