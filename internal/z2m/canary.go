package z2m

import "time"

const (
	canaryFactor = 3
	canaryMargin = 300 * time.Millisecond

	MinCanaryReads = 3
)

// CanaryLimit is how slow a far-away device may answer before the network counts as busy.
func CanaryLimit(rest time.Duration) time.Duration {
	return max(canaryFactor*rest, rest+canaryMargin)
}
