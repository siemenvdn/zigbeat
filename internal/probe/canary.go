package probe

import (
	"sync"
	"time"
)

// canaryProbe asks one far-away device for its state once per interval and times the answer.
type canaryProbe struct {
	mu       sync.Mutex
	device   string
	inflight bool
	sent     time.Time
	phase    string
	rate     float64
	rec      *recorder
	timeout  time.Duration
}

func (c *canaryProbe) tick(now time.Time, ph *phaseState, send func() error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inflight && now.Sub(c.sent) > c.timeout {
		c.rec.add(sample{sent: c.sent, phase: c.phase, rate: c.rate, kind: "canary", device: c.device, ok: false})
		c.inflight = false
	}
	if c.inflight {
		return
	}
	c.phase, c.rate = ph.get()
	c.sent = now
	c.inflight = true
	if err := send(); err != nil {
		c.inflight = false
	}
}

func (c *canaryProbe) onState(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.inflight {
		return
	}
	c.rec.add(sample{sent: c.sent, phase: c.phase, rate: c.rate, kind: "canary", device: c.device, latency: at.Sub(c.sent), ok: true})
	c.inflight = false
}

// recentOK reports the latencies of the last n answered canary probes.
func (c *canaryProbe) recentOK(n int) []time.Duration {
	samples, _ := c.rec.snapshot()
	var out []time.Duration
	for i := len(samples) - 1; i >= 0 && len(out) < n; i-- {
		s := samples[i]
		if s.kind == "canary" && s.device == c.device {
			if !s.ok {
				return nil
			}
			out = append(out, s.latency)
		}
	}
	return out
}
