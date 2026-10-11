package cli

import (
	"context"
	"fmt"
	"os"
	"time"
)

var redialEvery = 5 * time.Second

// Redial calls dial every 5 s until it works or ctx is done. Each failure goes
// to stderr after what.
func Redial(ctx context.Context, what string, dial func() error) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(redialEvery):
		}
		err := dial()
		if err == nil {
			return true
		}
		fmt.Fprintf(os.Stderr, "%s: %v\n", what, err)
	}
}
