package show

import "time"

// interval is the number of beats between moments: the smallest of 1, 2, 4, 8, 16
// that lasts at least the cadence.
func interval(bpm float64, cadence time.Duration) int {
	period := time.Duration(float64(time.Minute) / bpm)
	for _, n := range []int{1, 2, 4, 8} {
		if time.Duration(n)*period >= cadence {
			return n
		}
	}
	return 16
}
