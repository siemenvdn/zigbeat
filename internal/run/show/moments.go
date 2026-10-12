package show

import (
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
)

// intervals are the beats between moments to pick from: powers of two keep
// moments on bar lines.
var intervals = []int{1, 2, 4, 8, 16}

// interval is the number of beats between moments: the smallest of intervals
// that lasts at least the cadence, or the largest.
func interval(bpm float64, cadence time.Duration) int {
	period := music.Track{BPM: bpm}.Period()
	for _, n := range intervals {
		if time.Duration(n)*period >= cadence {
			return n
		}
	}
	return intervals[len(intervals)-1]
}

// spacing returns the beats between moments in force at index, or 0 when
// index is no moment. A new interval, from the pace or the tempo, starts only
// on a moment of both the old and the new grid, so the breathe fade planned at
// a moment always ends on the next one.
func (p *Planner) spacing(index int) int {
	want := interval(p.bpm, p.cadence)
	if p.every == 0 {
		p.every = want
	}
	if index%p.every != 0 {
		return 0
	}
	if index%want == 0 {
		p.every = want
	}
	return p.every
}
