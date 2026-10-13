package run

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/siemenvdn/zigbeat/internal/lights"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

const cancelled = "cancelled before the show started, nothing was changed"

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
		fmt.Println(cancelled)
		return 0, 0, false
	}
	if !snapped {
		return 0, 1, false
	}
	return limit, 0, true
}

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
				return fmt.Errorf("%w; fix the config or run zigbeat setup again", err)
			}
		}
	}
	return nil
}

const (
	canaryReads = 5
	canaryGap   = time.Second
)

func (r *runner) canaryBaseline(ctx context.Context) time.Duration {
	if r.cfg.Canary == "" {
		return 0
	}
	lat := lights.ReadTimes(ctx, r.link, r.cfg.Canary, canaryReads, canaryGap)
	if ctx.Err() != nil {
		return 0
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
