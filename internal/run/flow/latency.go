package flow

import (
	"slices"
	"time"
)

const (
	samples = 20 // latencies kept for the median
	// minSamples is the fewest latencies whose median replaces budget.latency_ms.
	minSamples = 3
	// risingFactor times the show's baseline is a rising latency: a queue builds up.
	risingFactor = 2
)

// record keeps the latency d of a confirmed flight: per frame of the message,
// and per frame of the queue it joined. The first samples of the latter make
// the baseline.
func (f *InFlight) record(fl flight, d time.Duration) {
	f.lat = keep(f.lat, d/time.Duration(fl.frames))
	f.served = keep(f.served, d/time.Duration(fl.queued))
	if f.base == 0 && len(f.served) == samples {
		f.base = median(f.served)
	}
}

// keep adds d to the last samples.
func keep(s []time.Duration, d time.Duration) []time.Duration {
	s = append(s, d)
	if len(s) > samples {
		s = s[1:]
	}
	return s
}

// Rising reports a served latency above risingFactor times the show's own
// baseline. Per frame of the queue a message joined, because a moment's
// messages leave together and wait for each other, more in one phrase than in
// another; the baseline, not the probe's figure, because the probe sent evenly.
func (f *InFlight) Rising() bool {
	return f.base > 0 && f.Served() > risingFactor*f.base
}

// Served is the median latency per frame of the queue a message joined.
func (f *InFlight) Served() time.Duration { return median(f.served) }

// Forget drops the latency samples but keeps the baseline, so samples from
// before a pause cannot pause the show again after it.
func (f *InFlight) Forget() { f.lat, f.served = nil, nil }

// Median returns the median latency per frame and the number of samples.
func (f *InFlight) Median() (time.Duration, int) { return median(f.lat), len(f.lat) }

// median is the middle of s, or 0 for none.
func median(s []time.Duration) time.Duration {
	if len(s) == 0 {
		return 0
	}
	s = slices.Clone(s)
	slices.Sort(s)
	return s[len(s)/2]
}

// Estimate is the latency per frame to send early by: the median once there are minSamples.
func (f *InFlight) Estimate() time.Duration {
	if m, n := f.Median(); n >= minSamples {
		return m
	}
	return f.initial
}

// Base is the median of the show's first served samples; 0 until there are enough.
func (f *InFlight) Base() time.Duration { return f.base }
