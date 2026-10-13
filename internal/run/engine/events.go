package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

// Music handles one event from the source. A Clear drops the moments planned
// from earlier beats; a Track goes to the planner; a Beat is planned when it is
// a moment. Beats already past are skipped, so a reconnect does not cause a
// burst; a pause plans nothing.
func (e *Engine) Music(now time.Time, ev music.Event) {
	switch ev := ev.(type) {
	case music.Clear:
		e.forget()
	case music.Track:
		e.plan.Track(ev)
	case music.Beat:
		if e.stopping || e.bo.Level() >= flow.PauseLevel || ev.At.Before(now) {
			return
		}
		e.enqueue(e.plan.Beat(ev, e.bo.Level() >= flow.CalmLevel))
	}
	e.planNotes(now)
}

// planNotes stamps the planner's notes, such as the album colours and the variant.
func (e *Engine) planNotes(now time.Time) {
	for _, n := range e.plan.TakeNotes() {
		e.note(now, n)
	}
}

// State handles a state message from a light or the canary.
func (e *Engine) State(light string, at time.Time) {
	if light == e.canary.Name() {
		if e.canary.Answer(at) {
			e.raise(at, flow.PauseLevel, fmt.Sprintf("canary answered after %d ms", e.canary.Last().Milliseconds()))
		}
		return
	}
	l := e.lights[light]
	if l == nil { // after a reconnect the session also watches lights left out of the show
		return
	}
	// Any state means the light is alive.
	l.fails = 0
	if l.away {
		l.away = false
		e.note(at, light+" answers again; it is back in the show")
	}
	if !e.fl.Confirm(light, at) {
		return
	}
	if e.fl.Rising() {
		e.raise(at, flow.PauseLevel, fmt.Sprintf("latency p50 %d ms per queued frame, show started at %d ms", e.fl.Served().Milliseconds(), e.fl.Base().Milliseconds()))
	}
}

// ZigbeeError handles an error line from Zigbee2MQTT's log.
func (e *Engine) ZigbeeError(at time.Time, msg string) {
	e.errs++
	light := e.named(msg)
	if light == "" {
		e.raise(at, flow.CalmLevel, "Zigbee2MQTT error elsewhere: "+msg)
		return
	}
	// Only a signal: the error may be about an older message, so the slot and
	// the failure count are left to the echo or the z2m.LostAfter expiry.
	if e.Away(light) {
		return
	}
	e.raise(at, flow.PauseLevel, "Zigbee2MQTT error: "+msg)
}

// named returns the light or canary a Zigbee2MQTT message is about, if any.
func (e *Engine) named(msg string) string {
	for _, l := range e.order {
		if strings.Contains(msg, "'"+l+"'") {
			return l
		}
	}
	if e.canary.Name() != "" && strings.Contains(msg, "'"+e.canary.Name()+"'") {
		return e.canary.Name()
	}
	return ""
}

// raise signals the backoff, at most to level max, and notes a step up; at
// the pause level the queues empty.
func (e *Engine) raise(now time.Time, max int, why string) {
	if !e.bo.Signal(now, max) {
		return
	}
	if e.bo.Level() == flow.PauseLevel {
		e.paused(now, why)
		return
	}
	e.note(now, fmt.Sprintf("backoff %d: %s", e.bo.Level(), why))
}

func (e *Engine) paused(now time.Time, why string) {
	e.clearQueues(every)
	e.fl.Forget()
	e.note(now, "paused: "+why+"; the show resumes after "+cli.Seconds(flow.RecoverAfter)+" of quiet")
}
