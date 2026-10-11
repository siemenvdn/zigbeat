package run

import (
	"context"
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

// sleep waits d, or less when ctx is done; it reports whether ctx is still live.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(max(0, d))
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// waitState reads events until light reports a state after since, or d passes.
// Other events are dropped: the show is not running.
func (r *runner) waitState(ctx context.Context, light string, since time.Time, d time.Duration) (z2m.State, time.Time, bool) {
	deadline := time.After(d)
	for {
		select {
		case ev := <-r.events:
			if ev.light == light && ev.at.After(since) {
				return ev.st, ev.at, true
			}
		case <-deadline:
			return z2m.State{}, time.Time{}, false
		case <-r.link.Lost():
			return z2m.State{}, time.Time{}, false
		case <-ctx.Done():
			return z2m.State{}, time.Time{}, false
		}
	}
}

// refuseGroups stops the show before it starts when a light or the canary is a
// Zigbee2MQTT group. Without bridge/groups it warns and goes on.
func (r *runner) refuseGroups(ctx context.Context) error {
	groups, ok := r.link.Groups(ctx)
	if !ok {
		if ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "warning: no %s/bridge/groups; make sure no light in the config is a group\n", r.cfg.MQTT.BaseTopic)
		}
		return nil
	}
	return r.cfg.RejectGroups(groups)
}

// snapshot reads every light, one at a time within the budget. A light that does
// not answer is left out of the show: without its state it cannot be restored.
// It returns false when no light answered or ctx was cancelled.
func (r *runner) snapshot(ctx context.Context) bool {
	r.snaps = map[string]z2m.State{}
	for _, l := range r.cfg.Lights() {
		for attempt := 0; attempt < 2; attempt++ {
			sent := time.Now()
			_ = r.link.Get(l, z2m.SnapshotQuery)
			st, _, ok := r.waitState(ctx, l, sent, 3*time.Second)
			if !sleep(ctx, r.pace(1)-time.Since(sent)) {
				return false
			}
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
func (r *runner) canaryBaseline(ctx context.Context) time.Duration {
	if r.cfg.Canary == "" {
		return 0
	}
	var lat []time.Duration
	for i := 0; i < 5; i++ {
		sent := time.Now()
		_ = r.link.Get(r.cfg.Canary, map[string]string{"state": ""})
		if _, at, ok := r.waitState(ctx, r.cfg.Canary, sent, 5*time.Second); ok {
			lat = append(lat, at.Sub(sent))
		}
		if !sleep(ctx, time.Second-time.Since(sent)) {
			return 0
		}
	}
	if len(lat) < 3 {
		fmt.Fprintf(os.Stderr, "warning: canary %s answered %d of 5 reads; running without a canary\n", r.cfg.Canary, len(lat))
		r.cfg.Canary = ""
		return 0
	}
	slowest := slices.Max(lat)
	limit := z2m.CanaryLimit(slowest)
	fmt.Printf("canary %s at rest: slowest %d ms (backoff above %d ms)\n", r.cfg.Canary, slowest.Milliseconds(), limit.Milliseconds())
	return limit
}

// restore puts every light of the show back, one at a time within the budget.
// When the broker drops it dials again for up to 30 s; it returns false when the
// broker stayed away, after printing what is left to restore by hand.
func (r *runner) restore() bool {
	fmt.Println("restoring lights")
	lights := r.cfg.Lights()
	failed := 0
	for i, l := range lights {
		ok, online := r.restoreLight(l)
		if !online {
			r.restoreByHand(lights[i:])
			return false
		}
		if !ok {
			failed++
			fmt.Printf("  could not confirm %s, set it back by hand\n", l)
		}
	}
	if failed == 0 {
		fmt.Println("  all lights back to their previous state")
	}
	return true
}

// restoreLight sends the restore payloads of l in order, each confirmed by a
// state, three tries each. online is false when the broker could not be reached.
func (r *runner) restoreLight(l string) (ok, online bool) {
	for _, p := range z2m.RestorePayloads(r.snaps[l]) {
		ok = false
		for attempt := 0; attempt < 3 && !ok; attempt++ {
			if !r.online() {
				return false, false
			}
			sent := time.Now()
			_ = r.link.Set(l, p)
			_, _, ok = r.waitState(context.Background(), l, sent, 3*time.Second)
			sleep(context.Background(), r.pace(z2m.Frames(p))-time.Since(sent))
		}
		if !ok {
			return false, true
		}
	}
	return true, true
}

// online reports whether the broker is there, dialling again for up to 30 s when it is not.
func (r *runner) online() bool {
	if !r.offline() {
		return true
	}
	fmt.Printf("%s  mqtt connection lost; reconnecting for up to 30 s\n", stamp())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return r.link.Reconnect(ctx)
}

func (r *runner) restoreByHand(lights []string) {
	left := map[string]z2m.State{}
	for _, l := range lights {
		left[l] = r.snaps[l]
	}
	b, _ := json.Marshal(left)
	fmt.Fprintf(os.Stderr, "mqtt is gone, restore by hand from: %s\n", b)
}
