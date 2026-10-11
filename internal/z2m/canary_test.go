package z2m

import (
	"testing"
	"time"
)

func TestCanaryLimit(t *testing.T) {
	if got := CanaryLimit(50 * time.Millisecond); got != 350*time.Millisecond {
		t.Errorf("a fast canary gets 300 ms of room: %s", got)
	}
	if got := CanaryLimit(200 * time.Millisecond); got != 600*time.Millisecond {
		t.Errorf("a slow canary may take 3 times as long: %s", got)
	}
}
