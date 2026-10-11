package probe

import (
	"sync"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

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

func (t *cmdTracker) busy(dev string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.open[dev]) > 0
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
