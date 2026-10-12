package z2m

import "time"

const (
	canaryFactor = 3                      // a busy mesh queues frames and answers several times slower
	canaryMargin = 300 * time.Millisecond // a tripled tiny latency is still noise, not a busy network
	// MinCanaryReads is the fewest answers at rest that give a usable limit.
	MinCanaryReads = 3
)

// CanaryLimit is how slow a far-away device may answer before the network counts
// as busy: canaryFactor times its resting latency, and at least canaryMargin more.
func CanaryLimit(rest time.Duration) time.Duration {
	return max(canaryFactor*rest, rest+canaryMargin)
}
