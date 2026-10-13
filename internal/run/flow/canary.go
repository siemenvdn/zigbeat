package flow

import (
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// CanaryEvery sees a busy network within seconds, for one frame of the budget.
const CanaryEvery = 5 * time.Second

// Canary reads a far-away device now and then to see how the rest of the network is doing.
func NewCanary(name string, limit time.Duration, now time.Time) Canary {
	return Canary{name: name, limit: limit, next: now}
}

type Canary struct {
	name  string
	limit time.Duration
	next  time.Time
	sent  time.Time
	wait  bool
	last  time.Duration
}

func (c *Canary) Due(now time.Time) bool {
	return c.name != "" && !c.wait && !now.Before(c.next)
}

func (c *Canary) Sent(now time.Time) {
	c.wait, c.sent, c.next = true, now, now.Add(CanaryEvery)
}

// Answer records a state from the canary and reports whether it was too slow.
func (c *Canary) Answer(at time.Time) bool {
	if !c.wait || !at.After(c.sent) {
		return false
	}
	c.wait = false
	c.last = at.Sub(c.sent)
	return c.last > c.limit
}

// Expire reports a read that stayed unanswered for z2m.LostAfter.
func (c *Canary) Expire(now time.Time) bool {
	if c.wait && now.Sub(c.sent) >= z2m.LostAfter {
		c.wait = false
		return true
	}
	return false
}

func (c *Canary) Name() string        { return c.name }
func (c *Canary) Waiting() bool       { return c.wait }
func (c *Canary) Last() time.Duration { return c.last }

// Forget drops a read that was sent on a connection that is gone.
func (c *Canary) Forget() { c.wait = false }
