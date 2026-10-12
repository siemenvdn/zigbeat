// Package engine is the show as a state machine: the loop feeds it events and
// the current time, and publishes what Due returns. It does no I/O itself.
package engine

import (
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/run/show"
)

const inFlightLimit = 3 // messages in flight in total, one moment's worth

// Send is one message for the loop to publish.
type Send struct {
	Light   string
	Get     bool
	Payload map[string]any
}

// Engine is one show. Only the loop goroutine may call it.
type Engine struct {
	plan     *show.Planner
	lim      *flow.Limiter
	bo       flow.Backoff
	fl       *flow.InFlight
	canary   flow.Canary
	lights   []string
	queue    map[string][]queued // per light, by send time
	started  map[int]bool        // units with a message out
	fails    map[string]int      // failures in a row per light
	away     map[string]time.Time
	offset   time.Duration
	latency  time.Duration // budget.latency_ms
	slowSeen bool          // the slow-network note was printed
	stopping bool
	notes    []string

	frames, errs, dropped int
	lastStatus            time.Time
}

func millis(n int) time.Duration { return time.Duration(n) * time.Millisecond }

// New starts a show from c and queues its start pass.
func New(c config.Config, canaryLim time.Duration, now time.Time) *Engine {
	e := &Engine{
		plan:       show.NewPlanner(c),
		lim:        flow.NewLimiter(c.Budget.MaxRate, now),
		fl:         flow.NewInFlight(millis(c.Budget.LatencyMS)),
		canary:     flow.NewCanary(c.Canary, canaryLim, now),
		lights:     c.Lights(),
		queue:      map[string][]queued{},
		started:    map[int]bool{},
		fails:      map[string]int{},
		away:       map[string]time.Time{},
		offset:     c.Budget.AudioOffset(),
		latency:    millis(c.Budget.LatencyMS),
		lastStatus: now,
	}
	e.enqueue(e.plan.Start(now))
	return e
}

// SetOffset sets the audio offset for every message planned from now on.
func (e *Engine) SetOffset(d time.Duration) { e.offset = d }

// Halt stops planning and sending; messages in flight can still be confirmed.
func (e *Engine) Halt() {
	e.stopping = true
	clear(e.queue)
}

// Disconnected forgets what was in flight on a connection that is gone.
func (e *Engine) Disconnected() {
	e.fl.Clear()
	clear(e.queue)
	e.canary.Forget()
}

func (e *Engine) InFlight() int {
	n := e.fl.Count()
	if e.canary.Waiting() {
		n++
	}
	return n
}

func (e *Engine) maxInFlight() int {
	if e.bo.Level() >= 1 {
		return 1
	}
	return inFlightLimit
}

func (e *Engine) note(now time.Time, s string) {
	e.notes = append(e.notes, now.Format(cli.Clock)+"  "+s)
}

// TakeNotes returns the lines to print since the last call.
func (e *Engine) TakeNotes() []string {
	n := e.notes
	e.notes = nil
	return n
}
