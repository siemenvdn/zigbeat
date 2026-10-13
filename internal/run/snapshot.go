package run

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/siemenvdn/zigbeat/internal/lights"
)

// snapshot reads every light within the budget. A light that does not answer is
// left out of the show: without its state it cannot be restored. It returns
// false when no light answered or ctx was cancelled.
func (r *runner) snapshot(ctx context.Context) bool {
	r.snaps = lights.Snapshot(ctx, restoreConn{r.link, r}, r.cfg.Lights(), r.cfg.Budget.MaxRate)
	if ctx.Err() != nil {
		return false
	}
	for _, l := range r.cfg.Lights() {
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
