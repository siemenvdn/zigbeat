package probe

import (
	"sync"
	"time"
)

// Canary asks one far-away device for its state once per interval and times the answer.
// NewCanary watches device; a read without answer after timeout is lost.
func NewCanary(device string, rec *Recorder, timeout time.Duration) *Canary {
	return &Canary{device: device, rec: rec, timeout: timeout}
}

type Canary struct {
	mu       sync.Mutex
	device   string
	inflight bool
	sent     time.Time
	phase    string
	rate     float64
	rec      *Recorder
	timeout  time.Duration
}

// Tick reads the canary unless a read is still open, and expires one that took too long.
func (c *Canary) Tick(now time.Time, ph *Phase, send func() error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inflight && now.Sub(c.sent) > c.timeout {
		c.rec.Add(Sample{Sent: c.sent, Phase: c.phase, Rate: c.rate, Kind: "canary", Device: c.device, OK: false})
		c.inflight = false
	}
	if c.inflight {
		return
	}
	c.phase, c.rate = ph.Get()
	c.sent = now
	c.inflight = true
	if err := send(); err != nil {
		c.inflight = false
	}
}

// OnState records the canary's answer.
func (c *Canary) OnState(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.inflight {
		return
	}
	c.rec.Add(Sample{Sent: c.sent, Phase: c.phase, Rate: c.rate, Kind: "canary", Device: c.device, Latency: at.Sub(c.sent), OK: true})
	c.inflight = false
}

// RecentOK reports the latencies of the last n answered canary probes.
func (c *Canary) RecentOK(n int) []time.Duration {
	samples, _ := c.rec.Snapshot()
	var out []time.Duration
	for i := len(samples) - 1; i >= 0 && len(out) < n; i-- {
		s := samples[i]
		if s.Kind == "canary" && s.Device == c.device {
			if !s.OK {
				return nil
			}
			out = append(out, s.Latency)
		}
	}
	return out
}
