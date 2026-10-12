package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

// slowFactor times budget.latency_ms at the show's start means the probe's
// figure no longer fits the network.
const slowFactor = 3

// Beat plans b if it is a moment. Beats already past are skipped, so a
// reconnect does not cause a burst; a pause plans nothing.
func (e *Engine) Beat(now time.Time, b beat.Beat) {
	if e.stopping || e.bo.Level() >= flow.PauseLevel || b.At.Before(now) {
		return
	}
	e.enqueue(e.plan.Beat(b, e.bo.Level()))
}

// State handles a state message from a light or the canary.
func (e *Engine) State(light string, at time.Time) {
	if light == e.canary.Name() {
		if e.canary.Answer(at) {
			e.raise(at, flow.PauseLevel, fmt.Sprintf("canary answered after %d ms", e.canary.Last().Milliseconds()))
		}
		return
	}
	// Any state means the light is alive.
	e.fails[light] = 0
	if _, gone := e.away[light]; gone {
		delete(e.away, light)
		e.note(at, light+" answers again; it is back in the show")
	}
	if !e.fl.Confirm(light, at) {
		return
	}
	if !e.slowSeen && e.fl.Base() > slowFactor*e.latency {
		e.slowSeen = true
		e.note(at, fmt.Sprintf("latency %d ms per frame is far above budget.latency_ms %d ms; run zigbeat probe again",
			e.fl.Base().Milliseconds(), e.latency.Milliseconds()))
	}
	if e.fl.Rising() {
		m, _ := e.fl.Median()
		e.raise(at, flow.PauseLevel, fmt.Sprintf("latency p50 %d ms per frame, show started at %d ms", m.Milliseconds(), e.fl.Base().Milliseconds()))
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
	// the failure count are left to the echo or the flow.LostAfter expiry.
	if _, gone := e.away[light]; gone {
		return
	}
	if e.bo.NamedError(at) {
		e.bo.Pause(at)
		e.paused(at, fmt.Sprintf("%d Zigbee2MQTT errors about our lights within %s", flow.NamedLimit, cli.Seconds(flow.NamedWindow)))
		return
	}
	e.raise(at, flow.PauseLevel, "Zigbee2MQTT error: "+msg)
}

// named returns the light or canary a Zigbee2MQTT message is about, if any.
func (e *Engine) named(msg string) string {
	for _, l := range e.lights {
		if strings.Contains(msg, "'"+l+"'") {
			return l
		}
	}
	if e.canary.Name() != "" && strings.Contains(msg, "'"+e.canary.Name()+"'") {
		return e.canary.Name()
	}
	return ""
}
