package probe

import (
	"sync"
	"time"

	"github.com/siemenvdn/zigbeat/internal/probe/palette"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Pending is a command waiting for its echo.
type Pending struct {
	Sent  time.Time
	Phase string
	Rate  float64
}

// Tracker matches state echoes to commands via a value that is unique per device within the timeout window
// (the colour x or the brightness).
type Tracker struct {
	mu      sync.Mutex
	open    map[string]map[int][]Pending
	rec     *Recorder
	timeout time.Duration
	keyOf   func(z2m.State) (int, bool)
}

func NewTracker(rec *Recorder, timeout time.Duration, keyOf func(z2m.State) (int, bool)) *Tracker {
	return &Tracker{open: map[string]map[int][]Pending{}, rec: rec, timeout: timeout, keyOf: keyOf}
}

func (t *Tracker) Add(dev string, key int, p Pending) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.open[dev] == nil {
		t.open[dev] = map[int][]Pending{}
	}
	t.open[dev][key] = append(t.open[dev][key], p)
}

func (t *Tracker) OnState(dev string, s z2m.State, at time.Time) {
	key, ok := t.keyOf(s)
	if !ok {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.open[dev]
	for d := range palette.KeySlack + 1 {
		for _, k := range []int{key - d, key + d} {
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
			t.rec.Add(Sample{Sent: p.Sent, Phase: p.Phase, Rate: p.Rate, Kind: "cmd", Device: dev, Latency: at.Sub(p.Sent), OK: true})
			return
		}
	}
}

func (t *Tracker) Expire(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for dev, m := range t.open {
		for k, q := range m {
			keep := q[:0]
			for _, p := range q {
				if now.Sub(p.Sent) > t.timeout {
					t.rec.Add(Sample{Sent: p.Sent, Phase: p.Phase, Rate: p.Rate, Kind: "cmd", Device: dev, OK: false})
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

func (t *Tracker) Busy(dev string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.open[dev]) > 0
}

func (t *Tracker) InFlight() int {
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
