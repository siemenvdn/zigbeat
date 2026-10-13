// Package engine is the show as a state machine.
package engine

import (
	"time"

	"golang.org/x/time/rate"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/run/show"
)

// Send is one message for the loop to publish.
type Send struct {
	Light   string
	Get     bool
	Payload map[string]any
}

// Engine is one show.
type Engine struct {
	plan     *show.Planner
	lim      *rate.Limiter
	bo       flow.Backoff
	fl       *flow.InFlight
	canary   flow.Canary
	order    []string
	lights   map[string]*light
	offset   time.Duration
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
		lim:        rate.NewLimiter(rate.Limit(c.Budget.MaxRate), flow.Burst),
		fl:         flow.NewInFlight(millis(c.Budget.LatencyMS)),
		canary:     flow.NewCanary(c.Canary, canaryLim, now),
		order:      c.Lights(),
		lights:     map[string]*light{},
		offset:     c.Budget.AudioOffset(),
		lastStatus: now,
	}
	for _, l := range e.order {
		e.lights[l] = &light{}
	}
	e.enqueue(e.plan.Start(now))
	return e
}

// SetOffset sets the audio offset for every message planned from now on.
func (e *Engine) SetOffset(d time.Duration) { e.offset = d }

// SetEffects takes the style of the show from the next moment on.
func (e *Engine) SetEffects(now time.Time, eff config.Effects) {
	e.plan.SetEffects(eff)
	e.planNotes(now)
}

// Halt stops planning and sending.
func (e *Engine) Halt() {
	e.stopping = true
	for _, l := range e.lights {
		l.queue = nil
	}
}

// Disconnected forgets what was in flight on a connection that is gone.
func (e *Engine) Disconnected() {
	e.fl.Clear()
	e.clearQueues(every)
	e.canary.Forget()
	for _, l := range e.lights {
		l.fails, l.retryAt = 0, time.Time{}
	}
}

func (e *Engine) InFlight() int {
	n := e.fl.Count()
	if e.canary.Waiting() {
		n++
	}
	return n
}

func (e *Engine) maxInFlight() int {
	if e.bo.Level() >= flow.CalmLevel {
		return 1
	}
	return flow.MaxInFlight
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
