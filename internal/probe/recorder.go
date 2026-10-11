package probe

import (
	"sync"
	"time"
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
