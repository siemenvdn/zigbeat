package run

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

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
