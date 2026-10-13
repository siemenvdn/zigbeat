package cli

import (
	"context"
	"fmt"
	"os"
	"time"
)

// RedialEvery is the wait between dials.
var RedialEvery = 5 * time.Second

// Redial calls dial every RedialEvery until it works or ctx is done.
func Redial(ctx context.Context, what string, dial func() error) bool {
	for {
		if !Sleep(ctx, RedialEvery) {
			return false
		}
		err := dial()
		if err == nil {
			return true
		}
		fmt.Fprintf(os.Stderr, "%s: %v\n", what, err)
	}
}
