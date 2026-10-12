package serve

import (
	"context"
	"fmt"
	"sync"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run"
	"github.com/siemenvdn/zigbeat/internal/run/engine"
	"github.com/siemenvdn/zigbeat/internal/serve/ha"
)

// server switches the show on and off for Home Assistant.
type server struct {
	show   func(ctx context.Context, h run.Hooks) int // run.Play in production
	cmds   chan bool                                  // switch commands, ON is true; newest wins
	styles <-chan config.Effects                      // the style after a choice in Home Assistant
	put    func(ha.State)                             // hands a state to the link; never blocks
	total  int                                        // lights in the config
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
		fmt.Printf("%s  waiting for the switch in Home Assistant\n", cli.Stamp())
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
// ignored while it runs. From the restore on, commands wait in cmds for the loop.
func (s *server) play(ctx context.Context) {
	fmt.Printf("%s  switched on in Home Assistant\n", cli.Stamp())
	s.put(ha.Starting)
	showCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	st := &states{put: s.put}
	unwatch := s.watchOff(st, cancel)
	defer unwatch()
	s.show(showCtx, run.Hooks{
		Effects: s.styles,
		Status:  func(stats engine.Stats) { st.playing(ha.Playing(stats, s.total)) },
		Stopping: func() string {
			unwatch()
			st.stopping()
			if ctx.Err() != nil {
				return cli.QuitHint
			}
			return ""
		},
	})
}

// watchOff cancels the show when OFF comes, until unwatch is called; unwatch
// returns when the watcher is gone.
func (s *server) watchOff(st *states, cancel func()) (unwatch func()) {
	quit, gone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(gone)
		for {
			select {
			case on := <-s.cmds:
				if !on {
					fmt.Printf("%s  switched off in Home Assistant\n", cli.Stamp())
					st.stopping()
					cancel()
					return
				}
			case <-quit:
				return
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(quit); <-gone }) }
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
