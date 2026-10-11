package run

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

const (
	lateLimit = 150 * time.Millisecond // a moment message this late is dropped
	awayAfter = 2                      // failures in a row before a light is left alone
	retryAway = time.Minute            // how often a light that is left alone is read
)

type queued struct {
	planned
	sendAt time.Time
}

// send is one message for the loop to publish.
type send struct {
	light   string
	get     bool
	payload map[string]any
}

// engine is the show as a state machine: the loop feeds it events and the
// current time, and publishes what due returns. It does no I/O itself.
type engine struct {
	plan     *planner
	lim      *limiter
	bo       backoff
	fl       *inflight
	canary   canaryWatch
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

func newEngine(c config.Config, canaryLim time.Duration, now time.Time) *engine {
	e := &engine{
		plan:       newPlanner(c),
		lim:        newLimiter(c.Budget.MaxRate, now),
		fl:         newInflight(millis(c.Budget.LatencyMS)),
		canary:     canaryWatch{name: c.Canary, limit: canaryLim, next: now},
		lights:     c.Lights(),
		queue:      map[string][]queued{},
		started:    map[int]bool{},
		fails:      map[string]int{},
		away:       map[string]time.Time{},
		offset:     millis(c.Budget.AudioOffsetMS),
		latency:    millis(c.Budget.LatencyMS),
		lastStatus: now,
	}
	e.enqueue(e.plan.start(now))
	return e
}

func (e *engine) enqueue(ps []planned) {
	est := e.fl.estimate()
	for _, p := range ps {
		if _, gone := e.away[p.light]; gone {
			continue
		}
		frames := time.Duration(max(1, z2m.Frames(p.payload)))
		q := queued{planned: p, sendAt: p.land.Add(e.offset - frames*est)}
		list := e.queue[p.light]
		i := 0
		for i < len(list) && !list[i].sendAt.After(q.sendAt) {
			i++
		}
		e.queue[p.light] = slices.Insert(list, i, q)
	}
}

// beat plans b if it is a moment. Beats already past are skipped, so a
// reconnect does not cause a burst; a pause plans nothing.
func (e *engine) beat(now time.Time, b beat.Beat) {
	if e.stopping || e.bo.level >= pauseLevel || b.At.Before(now) {
		return
	}
	e.enqueue(e.plan.beat(b, e.bo.level))
}

// state handles a state message from a light or the canary.
func (e *engine) state(light string, at time.Time) {
	if light == e.canary.name {
		if e.canary.answer(at) {
			e.raise(at, pauseLevel, fmt.Sprintf("canary answered after %d ms", e.canary.last.Milliseconds()))
		}
		return
	}
	// Any state means the light is alive.
	e.fails[light] = 0
	if _, gone := e.away[light]; gone {
		delete(e.away, light)
		e.note(at, light+" answers again; it is back in the show")
	}
	if !e.fl.confirm(light, at) {
		return
	}
	if !e.slowSeen && e.fl.base > 3*e.latency {
		e.slowSeen = true
		e.note(at, fmt.Sprintf("latency %d ms per frame is far above budget.latency_ms %d ms; run zigbeat probe again",
			e.fl.base.Milliseconds(), e.latency.Milliseconds()))
	}
	if e.fl.rising() {
		m, _ := e.fl.median()
		e.raise(at, pauseLevel, fmt.Sprintf("latency p50 %d ms per frame, show started at %d ms", m.Milliseconds(), e.fl.base.Milliseconds()))
	}
}

// zigbeeError handles an error line from Zigbee2MQTT's log.
func (e *engine) zigbeeError(at time.Time, msg string) {
	e.errs++
	light := e.named(msg)
	if light == "" {
		e.raise(at, 1, "Zigbee2MQTT error elsewhere: "+msg)
		return
	}
	// Only a signal: the error may be about an older message, so the slot and
	// the failure count are left to the echo or the 5 s expiry.
	if _, gone := e.away[light]; gone {
		return
	}
	if e.bo.namedError(at) {
		e.bo.pause(at)
		e.paused(at, "3 Zigbee2MQTT errors about our lights within a minute")
		return
	}
	e.raise(at, pauseLevel, "Zigbee2MQTT error: "+msg)
}

// named returns the light or canary a Zigbee2MQTT message is about, if any.
func (e *engine) named(msg string) string {
	for _, l := range e.lights {
		if strings.Contains(msg, "'"+l+"'") {
			return l
		}
	}
	if e.canary.name != "" && strings.Contains(msg, "'"+e.canary.name+"'") {
		return e.canary.name
	}
	return ""
}

// failed counts a lost or refused message. A light that fails awayAfter times in a
// row is left alone, so one light switched off at the wall cannot drag the rest of
// the show into backoff. It returns true when the light is left alone.
func (e *engine) failed(now time.Time, light string) bool {
	if _, gone := e.away[light]; gone {
		return true
	}
	e.fails[light]++
	if e.fails[light] < awayAfter {
		return false
	}
	e.away[light] = now.Add(retryAway)
	delete(e.queue, light)
	e.note(now, light+" does not answer; leaving it alone and retrying every minute")
	return true
}

func (e *engine) raise(now time.Time, max int, why string) {
	if !e.bo.signal(now, max) {
		return
	}
	if e.bo.level == pauseLevel {
		e.paused(now, why)
		return
	}
	e.note(now, fmt.Sprintf("backoff %d: %s", e.bo.level, why))
}

func (e *engine) paused(now time.Time, why string) {
	clear(e.queue)
	e.fl.forget()
	e.note(now, "paused: "+why+"; the show resumes after a quiet minute")
}

func (e *engine) note(now time.Time, s string) {
	e.notes = append(e.notes, now.Format("15:04:05")+"  "+s)
}

// takeNotes returns the lines to print since the last call.
func (e *engine) takeNotes() []string {
	n := e.notes
	e.notes = nil
	return n
}

// halt stops planning and sending; messages in flight can still be confirmed.
func (e *engine) halt() {
	e.stopping = true
	clear(e.queue)
}

// disconnected forgets what was in flight on a connection that is gone.
func (e *engine) disconnected() {
	clear(e.fl.open)
	clear(e.queue)
	e.canary.wait = false
}

func (e *engine) inFlight() int {
	n := e.fl.count()
	if e.canary.wait {
		n++
	}
	return n
}

func (e *engine) maxInFlight() int {
	if e.bo.level >= 1 {
		return 1
	}
	return maxPerMoment
}

// due expires old messages and returns what must be published now.
func (e *engine) due(now time.Time) []send {
	for _, l := range e.fl.expire(now) {
		if !e.failed(now, l) {
			e.raise(now, pauseLevel, l+" did not answer within 5 s")
		}
	}
	if e.canary.expire(now) {
		e.raise(now, pauseLevel, "canary did not answer within 5 s")
	}
	if e.bo.tick(now) {
		e.note(now, fmt.Sprintf("backoff %d: quiet for a minute", e.bo.level))
	}
	if e.stopping {
		return nil
	}
	e.dropLate(now)

	var out []send
	if e.canary.due(now) && e.inFlight() < e.maxInFlight() && e.lim.take(now, 1) {
		e.canary.sentAt(now)
		out = append(out, send{light: e.canary.name, get: true, payload: map[string]any{"state": ""}})
	}
	if e.bo.level >= pauseLevel {
		return out
	}
	out = append(out, e.retryAway(now)...)
	for _, l := range e.ready(now) {
		if e.inFlight() >= e.maxInFlight() {
			break
		}
		q := e.queue[l]
		n := 0
		for n < len(q) && !q[n].sendAt.After(now) {
			n++
		}
		payload := map[string]any{}
		for _, m := range q[:n] {
			maps.Copy(payload, m.payload)
		}
		frames := z2m.Frames(payload)
		if !e.lim.take(now, frames) {
			break
		}
		for _, m := range q[:n] {
			if m.unit > 0 {
				e.started[m.unit] = true
			}
		}
		e.queue[l] = q[n:]
		e.fl.add(l, now, echoes(payload), frames)
		e.frames += frames
		out = append(out, send{light: l, payload: payload})
	}
	return out
}

// retryAway reads each light that is left alone once per retryAway.
func (e *engine) retryAway(now time.Time) []send {
	var out []send
	for _, l := range e.lights {
		next, gone := e.away[l]
		if !gone || now.Before(next) || e.fl.busy(l) || e.inFlight() >= e.maxInFlight() || !e.lim.take(now, 1) {
			continue
		}
		e.away[l] = now.Add(retryAway)
		e.fl.add(l, now, true, 1)
		out = append(out, send{light: l, get: true, payload: map[string]any{"state": ""}})
	}
	return out
}

// ready lists the free lights with a due message, oldest send time first.
func (e *engine) ready(now time.Time) []string {
	var ls []string
	for _, l := range e.lights {
		if q := e.queue[l]; len(q) > 0 && !q[0].sendAt.After(now) && !e.fl.busy(l) {
			ls = append(ls, l)
		}
	}
	slices.SortStableFunc(ls, func(a, b string) int {
		return e.queue[a][0].sendAt.Compare(e.queue[b][0].sendAt)
	})
	return ls
}

// dropLate removes moment units that can no longer land on time, as long as
// none of their messages has left.
func (e *engine) dropLate(now time.Time) {
	late := map[int]bool{}
	for _, q := range e.queue {
		for _, m := range q {
			if m.drop && !e.started[m.unit] && now.Sub(m.sendAt) > lateLimit {
				late[m.unit] = true
			}
		}
	}
	if len(late) == 0 {
		return
	}
	e.dropped += len(late)
	for l, q := range e.queue {
		e.queue[l] = slices.DeleteFunc(q, func(m queued) bool { return late[m.unit] })
	}
}

// echoes reports whether Zigbee2MQTT answers this payload with a state.
func echoes(p map[string]any) bool {
	for _, k := range []string{"state", "brightness", "color"} {
		if _, ok := p[k]; ok {
			return true
		}
	}
	return false
}
