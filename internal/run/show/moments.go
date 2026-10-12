package show

import "time"

// intervals are the beats between moments to pick from: powers of two keep
// moments on bar lines.
var intervals = []int{1, 2, 4, 8, 16}

// interval is the number of beats between moments: the smallest of intervals
// that lasts at least the cadence, or the largest.
func interval(bpm float64, cadence time.Duration) int {
	period := time.Duration(float64(time.Minute) / bpm)
	for _, n := range intervals {
		if time.Duration(n)*period >= cadence {
			return n
		}
	}
	return intervals[len(intervals)-1]
}
