package probe

import (
	"sync"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

type sample struct {
	sent    time.Time
	phase   string
	rate    float64
	kind    string // cmd or canary
	device  string
	latency time.Duration
	ok      bool
}

type recorder struct {
	mu      sync.Mutex
	samples []sample
	logs    []logEvent
}

type logEvent struct {
	at    time.Time
	level string
	msg   string
}

func (r *recorder) add(s sample) {
	r.mu.Lock()
	r.samples = append(r.samples, s)
	r.mu.Unlock()
}

func (r *recorder) addLog(e logEvent) {
	r.mu.Lock()
	r.logs = append(r.logs, e)
	r.mu.Unlock()
}

func (r *recorder) snapshot() ([]sample, []logEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]sample(nil), r.samples...), append([]logEvent(nil), r.logs...)
}

type phaseState struct {
	mu   sync.Mutex
	name string
	rate float64
}

func (p *phaseState) set(name string, rate float64) {
	p.mu.Lock()
	p.name, p.rate = name, rate
	p.mu.Unlock()
}

func (p *phaseState) get() (string, float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.name, p.rate
}

type pending struct {
	sent  time.Time
	phase string
	rate  float64
}

// cmdTracker matches state echoes to commands via a value that is unique per device
// within the timeout window (the colour x or the brightness).
type cmdTracker struct {
	mu      sync.Mutex
	open    map[string]map[int][]pending
	rec     *recorder
	timeout time.Duration
	keyOf   func(z2m.State) (int, bool)
}

func newCmdTracker(rec *recorder, timeout time.Duration, keyOf func(z2m.State) (int, bool)) *cmdTracker {
	return &cmdTracker{open: map[string]map[int][]pending{}, rec: rec, timeout: timeout, keyOf: keyOf}
}

func (t *cmdTracker) add(dev string, key int, p pending) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.open[dev] == nil {
		t.open[dev] = map[int][]pending{}
	}
	t.open[dev][key] = append(t.open[dev][key], p)
}

func (t *cmdTracker) onState(dev string, s z2m.State, at time.Time) {
	key, ok := t.keyOf(s)
	if !ok {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.open[dev]
	for _, k := range []int{key, key - 1, key + 1} {
		q := m[k]
		if len(q) == 0 {
			continue
		}
		p := q[0]
		if len(q) == 1 {
			delete(m, k)
		} else {
			m[k] = q[1:]
		}
		t.rec.add(sample{sent: p.sent, phase: p.phase, rate: p.rate, kind: "cmd", device: dev, latency: at.Sub(p.sent), ok: true})
		return
	}
}

func (t *cmdTracker) expire(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for dev, m := range t.open {
		for k, q := range m {
			keep := q[:0]
			for _, p := range q {
				if now.Sub(p.sent) > t.timeout {
					t.rec.add(sample{sent: p.sent, phase: p.phase, rate: p.rate, kind: "cmd", device: dev, ok: false})
				} else {
					keep = append(keep, p)
				}
			}
			if len(keep) == 0 {
				delete(m, k)
			} else {
				m[k] = keep
			}
		}
	}
}

func (t *cmdTracker) inflight() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, m := range t.open {
		for _, q := range m {
			n += len(q)
		}
	}
	return n
}

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
