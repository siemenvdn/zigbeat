package probe

import (
	"sync"
	"time"
)

// Sample is one command or canary read and its answer.
type Sample struct {
	Sent    time.Time
	Phase   string
	Rate    float64
	Kind    string // cmd or canary
	Device  string
	Latency time.Duration
	OK      bool
}

// Recorder collects samples and Zigbee2MQTT log lines from several goroutines.
type Recorder struct {
	mu      sync.Mutex
	samples []Sample
	logs    []LogEvent
}

// LogEvent is a warning or error line from Zigbee2MQTT.
type LogEvent struct {
	At    time.Time
	Level string
	Msg   string
}

func (r *Recorder) Add(s Sample) {
	r.mu.Lock()
	r.samples = append(r.samples, s)
	r.mu.Unlock()
}

func (r *Recorder) AddLog(e LogEvent) {
	r.mu.Lock()
	r.logs = append(r.logs, e)
	r.mu.Unlock()
}

// Snapshot copies what was recorded so far.
func (r *Recorder) Snapshot() ([]Sample, []LogEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Sample(nil), r.samples...), append([]LogEvent(nil), r.logs...)
}

// Phase is what the probe is doing now; samples are tagged with it.
type Phase struct {
	mu   sync.Mutex
	name string
	rate float64
}

func NewPhase(name string) *Phase { return &Phase{name: name} }

func (p *Phase) Set(name string, rate float64) {
	p.mu.Lock()
	p.name, p.rate = name, rate
	p.mu.Unlock()
}

func (p *Phase) Get() (string, float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.name, p.rate
}
