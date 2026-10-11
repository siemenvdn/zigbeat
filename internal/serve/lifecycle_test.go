package serve

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/run"
	"github.com/siemenvdn/zigbeat/internal/run/engine"
	"github.com/siemenvdn/zigbeat/internal/serve/ha"
)

// fakeShow plays until ctx is done, then holds its restore until release gets a token.
type fakeShow struct {
	release chan struct{}
	start   chan struct{} // when set, the start waits for it to close
	code    int           // when not 0, returned at once, like a show that could not start
}

func (f *fakeShow) run(ctx context.Context, h run.Hooks) int {
	if f.code != 0 {
		return f.code
	}
	if f.start != nil {
		<-f.start
	}
	if ctx.Err() != nil {
		return 0 // like run.Play: cancelled before the show started, nothing was changed
	}
	h.Status(engine.Stats{Lights: 2})
	<-ctx.Done()
	h.Stopping()
	<-f.release
	return 0
}

type harness struct {
	cmds   chan bool
	got    chan string
	show   *fakeShow
	cancel context.CancelFunc
	done   chan struct{}
}

func serveFake(t *testing.T, code int) *harness {
	t.Helper()
	h := &harness{cmds: make(chan bool, 1), got: make(chan string, 64), done: make(chan struct{}),
		show: &fakeShow{release: make(chan struct{}, 4), code: code}}
	var mu sync.Mutex
	last := ""
	put := func(st ha.State) { // called from several goroutines, like link.put
		mu.Lock()
		defer mu.Unlock()
		if st.Status != last { // a repeated state is harmless; the tests check the order
			h.got <- st.Status
		}
		last = st.Status
	}
	s := &server{show: h.show.run, cmds: h.cmds, put: put, total: 2}
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go func() { s.loop(ctx); close(h.done) }()
	t.Cleanup(func() {
		cancel()
		h.show.release <- struct{}{}
		<-h.done
	})
	h.expect(t, "off")
	return h
}

func (h *harness) expect(t *testing.T, want ...string) {
	t.Helper()
	for _, w := range want {
		select {
		case g := <-h.got:
			if g != w {
				t.Fatalf("status %q, want %q", g, w)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no status, want %q", w)
		}
	}
}

func (h *harness) quiet(t *testing.T) {
	t.Helper()
	select {
	case g := <-h.got:
		t.Fatalf("unexpected status %q", g)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestOnAndOff(t *testing.T) {
	h := serveFake(t, 0)
	newest(h.cmds, true)
	h.expect(t, "starting", "running")
	newest(h.cmds, false)
	h.expect(t, "stopping")
	h.show.release <- struct{}{}
	h.expect(t, "off")
}

func TestRepeatedCommandsChangeNothing(t *testing.T) {
	h := serveFake(t, 0)
	newest(h.cmds, false)
	h.quiet(t)
	newest(h.cmds, true)
	h.expect(t, "starting", "running")
	newest(h.cmds, true)
	h.quiet(t)
	newest(h.cmds, false)
	h.expect(t, "stopping")
}

func TestOnWhileStoppingStartsAgain(t *testing.T) {
	h := serveFake(t, 0)
	newest(h.cmds, true)
	h.expect(t, "starting", "running")
	newest(h.cmds, false)
	h.expect(t, "stopping")
	newest(h.cmds, true)
	h.show.release <- struct{}{}
	h.expect(t, "off", "starting", "running")
}

func TestOffDuringStartIsShownAtOnce(t *testing.T) {
	h := serveFake(t, 0)
	h.show.start = make(chan struct{})
	newest(h.cmds, true)
	h.expect(t, "starting")
	newest(h.cmds, false)
	h.expect(t, "stopping")
	close(h.show.start)
	h.expect(t, "off")
}

func TestFailedStartGoesBackToOff(t *testing.T) {
	h := serveFake(t, 1)
	newest(h.cmds, true)
	h.expect(t, "starting", "off")
}

func TestSignalStopsAndRestores(t *testing.T) {
	h := serveFake(t, 0)
	newest(h.cmds, true)
	h.expect(t, "starting", "running")
	h.cancel()
	h.expect(t, "stopping")
	select {
	case <-h.done:
		t.Fatal("serve must wait for the restore")
	case <-time.After(100 * time.Millisecond):
	}
	h.show.release <- struct{}{}
	h.expect(t, "off")
	<-h.done
}
