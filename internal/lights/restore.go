package lights

import (
	"context"
	"fmt"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Restore puts each light back to its state in snaps, one at a time within rate frames per second, and prints
// how it went.
func Restore(c Conn, snaps map[string]z2m.State, lights []string, rate float64) (left []string) {
	fmt.Println("restoring lights")
	failed := 0
	for i, l := range lights {
		ok, online := restore(c, l, snaps[l], rate)
		if !online {
			return lights[i:]
		}
		if !ok {
			failed++
			fmt.Printf("  could not confirm %s, set it back by hand\n", l)
		}
	}
	if failed == 0 {
		fmt.Println("  all lights back to their previous state")
	}
	return nil
}

const restoreTries = 3

func restore(c Conn, l string, s z2m.State, rate float64) (ok, online bool) {
	for _, p := range payloads(s) {
		ok = false
		for attempt := 0; attempt < restoreTries && !ok; attempt++ {
			if !c.Online() {
				return false, false
			}
			sent := time.Now()
			_ = c.Set(l, p)
			_, ok = c.Await(context.Background(), l, sent, answerWait)
			time.Sleep(pace(z2m.Frames(p), rate) - time.Since(sent))
		}
		if !ok {
			return false, true
		}
	}
	return true, true
}
