package cli

import (
	"context"
	"fmt"
	"os"
	"time"
)

// RedialEvery is the wait between dials: a restarted broker is back soon, and
// the log gets one line per try, not a flood.
var RedialEvery = 5 * time.Second

// Redial calls dial every RedialEvery until it works or ctx is done. Each failure goes
// to stderr after what.
func Redial(ctx context.Context, what string, dial func() error) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(RedialEvery):
		}
		err := dial()
		if err == nil {
			return true
		}
		fmt.Fprintf(os.Stderr, "%s: %v\n", what, err)
	}
}
