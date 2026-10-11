package run

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

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
	r.cfg = r.cfg.Keep(func(l string) bool { _, ok := r.snaps[l]; return ok })
	r.snapJSON, _ = json.Marshal(r.snaps)
	return true
}
