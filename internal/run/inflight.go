package run

import (
	"slices"
	"time"
)

const (
	lostAfter = 5 * time.Second // no echo after this: lost
	moveHold  = time.Second     // a brightness_move has no echo; its slot frees after this
	samples   = 20              // latencies kept for the median
)

type flight struct {
	sent   time.Time
	echo   bool // waits for a state echo
	frames int
}

// inflight holds at most one unconfirmed message per light and measures latency
// per Zigbee frame, the unit zigbeat probe measured in.
type inflight struct {
	open    map[string]flight
	lat     []time.Duration
	initial time.Duration
	base    time.Duration // median of the show's first samples; 0 until known
}

func newInflight(initial time.Duration) *inflight {
	return &inflight{open: map[string]flight{}, initial: initial}
}

func (f *inflight) add(light string, now time.Time, echo bool, frames int) {
	f.open[light] = flight{sent: now, echo: echo, frames: max(1, frames)}
}

func (f *inflight) busy(light string) bool {
	_, ok := f.open[light]
	return ok
}

func (f *inflight) count() int { return len(f.open) }

// confirm frees light on its first state after the send. It returns true when
// that confirmed an echo, whose latency per frame is then recorded.
func (f *inflight) confirm(light string, at time.Time) bool {
	fl, ok := f.open[light]
	if !ok || !at.After(fl.sent) {
		return false
	}
	delete(f.open, light)
	if !fl.echo {
		return false
	}
	f.lat = append(f.lat, at.Sub(fl.sent)/time.Duration(fl.frames))
	if len(f.lat) > samples {
		f.lat = f.lat[1:]
	}
	if f.base == 0 && len(f.lat) == samples {
		f.base, _ = f.median()
	}
	return true
}

// rising reports a median latency above twice the show's own baseline. The baseline,
// not the probe's figure, because moments send in bursts the probe never did.
func (f *inflight) rising() bool {
	m, _ := f.median()
	return f.base > 0 && m > 2*f.base
}

// forget drops the latency samples but keeps the baseline, so samples from
// before a pause cannot pause the show again after it.
func (f *inflight) forget() { f.lat = nil }

// fail frees light after Zigbee2MQTT reported an error for it.
func (f *inflight) fail(light string) { delete(f.open, light) }

// expire frees moves after moveHold and returns the lights whose echo did not
// come within lostAfter, sorted.
func (f *inflight) expire(now time.Time) []string {
	var lost []string
	for l, fl := range f.open {
		switch {
		case !fl.echo && now.Sub(fl.sent) >= moveHold:
			delete(f.open, l)
		case fl.echo && now.Sub(fl.sent) >= lostAfter:
			delete(f.open, l)
			lost = append(lost, l)
		}
	}
	slices.Sort(lost)
	return lost
}

// median returns the median latency per frame and the number of samples.
func (f *inflight) median() (time.Duration, int) {
	if len(f.lat) == 0 {
		return 0, 0
	}
	s := slices.Clone(f.lat)
	slices.Sort(s)
	return s[len(s)/2], len(s)
}

// estimate is the latency per frame to send early by: the median once there are 3 samples.
func (f *inflight) estimate() time.Duration {
	if m, n := f.median(); n >= 3 {
		return m
	}
	return f.initial
}
