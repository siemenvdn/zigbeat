package run

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/lights"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// restore puts every light back within the budget. It returns false when the
// broker stayed away, after printing what is left to restore by hand.
func (r *runner) restore() bool {
	left := lights.Restore(conn{r}, r.snaps, restoreOrder(r.cfg.Lights(), r.away), r.cfg.Budget.MaxRate)
	if len(left) > 0 {
		r.restoreByHand(left)
		return false
	}
	return true
}

// away reports whether the show left light alone.
func (r *runner) away(light string) bool { return r.eng != nil && r.eng.Away(light) }

// restoreOrder puts the lights that are left alone last: each takes all its
// tries, which would hold up the lights behind it.
func restoreOrder(ls []string, away func(string) bool) []string {
	var first, last []string
	for _, l := range ls {
		if away(l) {
			last = append(last, l)
		} else {
			first = append(first, l)
		}
	}
	return append(first, last...)
}

// restoreRedial is how long the restore dials a lost broker: a restart is back
// within it, and a stop does not seem to hang.
const restoreRedial = 30 * time.Second

// online reports whether the broker is there, dialling again for up to restoreRedial when it is not.
func (r *runner) online() bool {
	if !r.offline() {
		return true
	}
	cli.Say("mqtt connection lost; reconnecting for up to %s", cli.Seconds(restoreRedial))
	ctx, cancel := context.WithTimeout(context.Background(), restoreRedial)
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
