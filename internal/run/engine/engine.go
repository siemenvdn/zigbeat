// Package engine is the show as a state machine: the loop feeds it events and
// the current time, and publishes what Due returns. It does no I/O itself.
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

// Engine is one show. Only the loop goroutine may call it.
type Engine struct {
	plan     *show.Planner
	lim      *rate.Limiter // Zigbee frames; budget.max_rate is a ceiling, never raised
	bo       flow.Backoff
	fl       *flow.InFlight
	canary   flow.Canary
	order    []string          // the lights in config order
	lights   map[string]*light // per light: its queue, failures and whether it is left alone
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

// SetEffects takes the style of the show from the next moment on; it sends
// nothing itself.
func (e *Engine) SetEffects(now time.Time, eff config.Effects) {
	e.plan.SetEffects(eff)
	e.planNotes(now)
}

// Halt stops planning and sending; messages in flight can still be confirmed.
func (e *Engine) Halt() {
	e.stopping = true
	for _, l := range e.lights {
		l.queue = nil
	}
}

// Disconnected forgets what was in flight on a connection that is gone. The
// losses before a drop that the keepalive found late may be the connection's,
// so failure counts start over and lights left alone are read again at once.
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

// maxInFlight is one full moment in flight in total, or one message from the calm level.
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
