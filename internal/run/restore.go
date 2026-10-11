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
	left := lights.Restore(conn{r}, r.snaps, r.cfg.Lights(), r.cfg.Budget.MaxRate)
	if len(left) > 0 {
		r.restoreByHand(left)
		return false
	}
	return true
}

// online reports whether the broker is there, dialling again for up to 30 s when it is not.
func (r *runner) online() bool {
	if !r.offline() {
		return true
	}
	fmt.Printf("%s  mqtt connection lost; reconnecting for up to 30 s\n", cli.Stamp())
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
