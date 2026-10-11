package run

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// pace keeps one-at-a-time phases under the budget: frames / max_rate.
func (r *runner) pace(frames int) time.Duration {
	return time.Duration(float64(frames) / r.cfg.Budget.MaxRate * float64(time.Second))
}

// waitState reads events until light reports a state after since, or d passes.
// Other events are dropped: the show is not running.
func (r *runner) waitState(light string, since time.Time, d time.Duration) (z2m.State, time.Time, bool) {
	deadline := time.After(d)
	for {
		select {
		case ev := <-r.events:
			if ev.light == light && ev.at.After(since) {
				return ev.st, ev.at, true
			}
		case <-deadline:
			return z2m.State{}, time.Time{}, false
		case <-r.client.Done():
			return z2m.State{}, time.Time{}, false
		}
	}
}

// snapshot reads every light, one at a time within the budget. A light that does
// not answer is left out of the show: without its state it cannot be restored.
// It returns false when no light answered.
func (r *runner) snapshot() bool {
	r.snaps = map[string]z2m.State{}
	for _, l := range r.cfg.Lights() {
		for attempt := 0; attempt < 2; attempt++ {
			sent := time.Now()
			_ = r.bridge.Get(l, z2m.SnapshotQuery)
			st, _, ok := r.waitState(l, sent, 3*time.Second)
			time.Sleep(r.pace(1) - time.Since(sent))
			if ok {
				r.snaps[l] = st
				break
			}
		}
		if _, ok := r.snaps[l]; !ok {
			fmt.Fprintf(os.Stderr, "warning: no answer from %s, leaving it out of the show; check the friendly name and that it is powered\n", l)
		}
	}
	if len(r.snaps) == 0 {
		fmt.Fprintln(os.Stderr, "no light answered, nothing to do")
		return false
	}
	r.cfg = keepLights(r.cfg, r.snaps)
	r.snapJSON, _ = json.Marshal(r.snaps)
	return true
}

// keepLights returns c with only the lights in keep; fixtures left empty are dropped.
func keepLights(c config.Config, keep map[string]z2m.State) config.Config {
	var fs []config.Fixture
	for _, f := range c.Fixtures {
		var ls []string
		for _, l := range f.Lights {
			if _, ok := keep[l]; ok {
				ls = append(ls, l)
			}
		}
		if len(ls) > 0 {
			f.Lights = ls
			fs = append(fs, f)
		}
	}
	c.Fixtures = fs
	return c
}

// canaryBaseline reads the canary 5 times, 1 s apart, and returns its limit.
// A canary that does not answer is dropped with a warning.
func (r *runner) canaryBaseline() time.Duration {
	if r.cfg.Canary == "" {
		return 0
	}
	var lat []time.Duration
	for i := 0; i < 5; i++ {
		sent := time.Now()
		_ = r.bridge.Get(r.cfg.Canary, map[string]string{"state": ""})
		if _, at, ok := r.waitState(r.cfg.Canary, sent, 5*time.Second); ok {
			lat = append(lat, at.Sub(sent))
		}
		time.Sleep(time.Second - time.Since(sent))
	}
	if len(lat) < 3 {
		fmt.Fprintf(os.Stderr, "warning: canary %s answered %d of 5 reads; running without a canary\n", r.cfg.Canary, len(lat))
		r.cfg.Canary = ""
		return 0
	}
	slowest := slices.Max(lat)
	limit := canaryLimit(slowest)
	fmt.Printf("canary %s at rest: slowest %d ms (backoff above %d ms)\n", r.cfg.Canary, slowest.Milliseconds(), limit.Milliseconds())
	return limit
}

// restore puts every light of the show back, one at a time within the budget, three tries each.
func (r *runner) restore() {
	fmt.Println("restoring lights")
	failed := 0
	for _, l := range r.cfg.Lights() {
		p := z2m.RestorePayload(r.snaps[l])
		ok := false
		for attempt := 0; attempt < 3 && !ok; attempt++ {
			sent := time.Now()
			_ = r.bridge.Set(l, p)
			_, _, ok = r.waitState(l, sent, 3*time.Second)
			time.Sleep(r.pace(z2m.Frames(p)) - time.Since(sent))
		}
		if !ok {
			failed++
			fmt.Printf("  could not confirm %s, set it back by hand\n", l)
		}
	}
	if failed == 0 {
		fmt.Println("  all lights back to their previous state")
	}
}
