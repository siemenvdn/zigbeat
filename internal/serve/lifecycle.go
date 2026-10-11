package serve

import (
	"context"
	"fmt"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/run"
	"github.com/siemenvdn/zigbeat/internal/run/engine"
	"github.com/siemenvdn/zigbeat/internal/serve/ha"
)

// server switches the show on and off for Home Assistant.
type server struct {
	show  func(ctx context.Context, h run.Hooks) int // run.Play in production
	cmds  chan bool                                  // switch commands, ON is true; newest wins
	put   func(ha.State)                             // hands a state to the link; never blocks
	total int                                        // lights in the config
}

// newest puts v in a channel of one, replacing what is waiting there.
// Only one goroutine may send on ch.
func newest(ch chan bool, v bool) {
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
// ignored while it runs. A command that arrives during the restore waits in cmds
// for the loop.
func (s *server) play(ctx context.Context) {
	fmt.Printf("%s  switched on in Home Assistant\n", cli.Stamp())
	s.put(ha.Starting)
	showCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case on := <-s.cmds:
				if !on {
					fmt.Printf("%s  switched off in Home Assistant\n", cli.Stamp())
					s.put(ha.Stopping)
					cancel()
					return
				}
			case <-stop:
				return
			}
		}
	}()
	s.show(showCtx, run.Hooks{
		Status:   func(st engine.Stats) { s.put(ha.Playing(st, s.total)) },
		Stopping: func() { s.put(ha.Stopping) },
	})
	close(stop)
	<-stopped
}
