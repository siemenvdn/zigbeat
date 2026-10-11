package z2m

import "time"

// CanaryLimit is how slow a far-away device may answer before the network counts
// as busy: 3 times its resting latency, and at least 300 ms more.
func CanaryLimit(rest time.Duration) time.Duration {
	return max(3*rest, rest+300*time.Millisecond)
}
