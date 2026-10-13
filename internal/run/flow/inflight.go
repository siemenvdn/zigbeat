package flow

import (
	"slices"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

type flight struct {
	sent   time.Time
	frames int
	queued int
}

// InFlight holds at most one unconfirmed message per light and measures latency per Zigbee frame, the unit
// zigbeat probe measured in.
type InFlight struct {
	open    map[string]flight
	lat     []time.Duration
	served  []time.Duration
	initial time.Duration
	base    time.Duration
}

func NewInFlight(initial time.Duration) *InFlight {
	return &InFlight{open: map[string]flight{}, initial: initial}
}

// Add holds light's slot for a message of frames sent now, behind the frames already in flight.
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

// Confirm frees light on its first state after the send and records its latency per frame.
func (f *InFlight) Confirm(light string, at time.Time) bool {
	fl, ok := f.open[light]
	if !ok || !at.After(fl.sent) {
		return false
	}
	delete(f.open, light)
	f.record(fl, at.Sub(fl.sent))
	return true
}

// Expire returns the lights whose confirmation did not come within z2m.LostAfter, sorted.
func (f *InFlight) Expire(now time.Time) []string {
	var lost []string
	for l, fl := range f.open {
		if now.Sub(fl.sent) >= z2m.LostAfter {
			delete(f.open, l)
			lost = append(lost, l)
		}
	}
	slices.Sort(lost)
	return lost
}

// Clear forgets every message in flight, after the connection is gone.
func (f *InFlight) Clear() { clear(f.open) }
