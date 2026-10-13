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

func (r *runner) restore() bool {
	left := lights.Restore(restoreConn{r.link, r}, r.snaps, restoreOrder(r.cfg.Lights(), r.away), r.cfg.Budget.MaxRate)
	if len(left) > 0 {
		r.restoreByHand(left)
		return false
	}
	return true
}

func (r *runner) away(light string) bool { return r.eng != nil && r.eng.Away(light) }

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

const restoreRedial = 30 * time.Second

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
