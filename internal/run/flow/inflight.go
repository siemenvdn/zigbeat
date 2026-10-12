package flow

import (
	"slices"
	"time"
)

const (
	LostAfter = 5 * time.Second // no echo after this: lost
	samples   = 20              // latencies kept for the median
	// minSamples is the fewest latencies whose median replaces budget.latency_ms.
	minSamples = 3
	// risingFactor times the show's baseline is a rising latency: a queue builds up.
	risingFactor = 2
)

type flight struct {
	sent   time.Time
	frames int
}

// InFlight holds at most one unconfirmed message per light and measures latency
// per Zigbee frame, the unit zigbeat probe measured in.
type InFlight struct {
	open    map[string]flight
	lat     []time.Duration
	initial time.Duration
	base    time.Duration // Median of the show's first samples; 0 until known
}

func NewInFlight(initial time.Duration) *InFlight {
	return &InFlight{open: map[string]flight{}, initial: initial}
}

func (f *InFlight) Add(light string, now time.Time, frames int) {
	f.open[light] = flight{sent: now, frames: max(1, frames)}
}

func (f *InFlight) Busy(light string) bool {
	_, ok := f.open[light]
	return ok
}

func (f *InFlight) Count() int { return len(f.open) }

// Confirm frees light on its first state after the send and records its
// latency per frame. It returns false for a state that confirms nothing.
func (f *InFlight) Confirm(light string, at time.Time) bool {
	fl, ok := f.open[light]
	if !ok || !at.After(fl.sent) {
		return false
	}
	delete(f.open, light)
	f.lat = append(f.lat, at.Sub(fl.sent)/time.Duration(fl.frames))
	if len(f.lat) > samples {
		f.lat = f.lat[1:]
	}
	if f.base == 0 && len(f.lat) == samples {
		f.base, _ = f.Median()
	}
	return true
}

// Rising reports a median latency above risingFactor times the show's own baseline. The baseline,
// not the probe's figure, because moments send in bursts the probe never did.
func (f *InFlight) Rising() bool {
	m, _ := f.Median()
	return f.base > 0 && m > risingFactor*f.base
}

// Forget drops the latency samples but keeps the baseline, so samples from
// before a pause cannot pause the show again after it.
func (f *InFlight) Forget() { f.lat = nil }

// Expire returns the lights whose confirmation did not come within LostAfter, sorted.
func (f *InFlight) Expire(now time.Time) []string {
	var lost []string
	for l, fl := range f.open {
		if now.Sub(fl.sent) >= LostAfter {
			delete(f.open, l)
			lost = append(lost, l)
		}
	}
	slices.Sort(lost)
	return lost
}

// Median returns the median latency per frame and the number of samples.
func (f *InFlight) Median() (time.Duration, int) {
	if len(f.lat) == 0 {
		return 0, 0
	}
	s := slices.Clone(f.lat)
	slices.Sort(s)
	return s[len(s)/2], len(s)
}

// Estimate is the latency per frame to send early by: the median once there are minSamples.
func (f *InFlight) Estimate() time.Duration {
	if m, n := f.Median(); n >= minSamples {
		return m
	}
	return f.initial
}

// Base is the median of the show's first samples; 0 until there are enough.
func (f *InFlight) Base() time.Duration { return f.base }

// Clear forgets every message in flight, after the connection is gone.
func (f *InFlight) Clear() { clear(f.open) }
