package run

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// start checks the config against Zigbee2MQTT, takes the snapshot and measures
// the canary. Nothing is changed yet, so a stop here needs no restore.
func (r *runner) start(ctx context.Context) (limit time.Duration, code int, ok bool) {
	if err := r.refuse(ctx); err != nil {
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

// refuse runs refuseGroups and refuseDevices at once, so a Zigbee2MQTT without
// retain costs one RetainedWait, not two.
func (r *runner) refuse(ctx context.Context) error {
	var wg sync.WaitGroup
	refusals := []func(context.Context) error{r.refuseGroups, r.refuseDevices}
	errs := make([]error, len(refusals))
	for i, refuse := range refusals {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = refuse(ctx)
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}

// refuseGroups stops the show before it starts when a light or the canary is a
// Zigbee2MQTT group. Without bridge/groups it warns and goes on.
func (r *runner) refuseGroups(ctx context.Context) error {
	groups, err := r.link.Groups(ctx)
	if err != nil {
		if ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "warning: %v; make sure no light in the config is a group\n", err)
		}
		return nil
	}
	return r.cfg.RejectGroups(groups)
}

// refuseDevices stops the show before it starts when a light may not be driven
// in its role, by the rules of zigbeat setup: they also hold for a config
// edited by hand. Without bridge/devices it warns and goes on.
func (r *runner) refuseDevices(ctx context.Context) error {
	devs, err := r.link.Devices(ctx)
	if err != nil {
		if ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "warning: %v; the lights in the config were not checked against it\n", err)
		}
		return nil
	}
	for _, f := range r.cfg.Fixtures {
		for _, l := range f.Lights {
			if err := z2m.Check(devs, l, f.Role); err != nil {
				return err
			}
		}
	}
	return nil
}

const (
	canaryReads = 5           // reads at rest: the slowest sets the limit
	canaryGap   = time.Second // about as often as the probe's canary
)

// canaryBaseline reads the canary canaryReads times, canaryGap apart, and returns its limit.
// A canary that does not answer is dropped with a warning.
func (r *runner) canaryBaseline(ctx context.Context) time.Duration {
	if r.cfg.Canary == "" {
		return 0
	}
	var lat []time.Duration
	for range canaryReads {
		sent := time.Now()
		_ = r.link.Get(r.cfg.Canary, map[string]string{"state": ""})
		if _, at, ok := r.waitState(ctx, r.cfg.Canary, sent, flow.LostAfter); ok {
			lat = append(lat, at.Sub(sent))
		}
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(canaryGap - time.Since(sent)):
		}
	}
	if len(lat) < z2m.MinCanaryReads {
		fmt.Fprintf(os.Stderr, "warning: canary %s answered %d of %d reads; running without a canary\n", r.cfg.Canary, len(lat), canaryReads)
		r.cfg.Canary = ""
		return 0
	}
	slowest := slices.Max(lat)
	limit := z2m.CanaryLimit(slowest)
	fmt.Printf("canary %s at rest: slowest %d ms (backoff above %d ms)\n", r.cfg.Canary, slowest.Milliseconds(), limit.Milliseconds())
	return limit
}
