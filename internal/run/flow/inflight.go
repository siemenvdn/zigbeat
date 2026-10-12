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
	queued int // frames in flight when it left, its own included
}

// InFlight holds at most one unconfirmed message per light and measures latency
// per Zigbee frame, the unit zigbeat probe measured in.
type InFlight struct {
	open    map[string]flight
	lat     []time.Duration // per frame of the message: when to send early
	served  []time.Duration // per frame of the queue the message joined: how fast the coordinator is
	initial time.Duration
	base    time.Duration // median of the show's first served samples; 0 until known
}

func NewInFlight(initial time.Duration) *InFlight {
	return &InFlight{open: map[string]flight{}, initial: initial}
}

// Add holds light's slot for a message of frames sent now, behind the frames
// already in flight.
func (f *InFlight) Add(light string, now time.Time, frames int) {
	fl := flight{sent: now, frames: max(1, frames)}
	fl.queued = fl.frames
	for _, o := range f.open {
		fl.queued += o.frames
	}
	f.open[light] = fl
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
	d := at.Sub(fl.sent)
	f.lat = keep(f.lat, d/time.Duration(fl.frames))
	f.served = keep(f.served, d/time.Duration(fl.queued))
	if f.base == 0 && len(f.served) == samples {
		f.base = median(f.served)
	}
	return true
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

// Clear forgets every message in flight, after the connection is gone.
func (f *InFlight) Clear() { clear(f.open) }
