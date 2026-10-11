package run

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// start checks the config against Zigbee2MQTT, takes the snapshot and measures
// the canary. Nothing is changed yet, so a stop here needs no restore.
func (r *runner) start(ctx context.Context) (limit time.Duration, code int, ok bool) {
	if err := r.refuseGroups(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 0, 1, false
	}
	snapped := r.snapshot(ctx)
	if snapped {
		fmt.Printf("current state (for manual restore): %s\n", r.snapJSON)
		limit = r.canaryBaseline(ctx)
	}
	if ctx.Err() != nil {
		fmt.Println("cancelled before the show started, nothing was changed")
		return 0, 0, false
	}
	if !snapped {
		return 0, 1, false
	}
	return limit, 0, true
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
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(time.Second - time.Since(sent)):
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
