package run

import "time"

const canaryEvery = 5 * time.Second

// canaryWatch reads a far-away device now and then to see how the rest of the network is doing.
type canaryWatch struct {
	name  string
	limit time.Duration
	next  time.Time
	sent  time.Time
	wait  bool
	last  time.Duration
}

// canaryLimit is the probe's rule: 3 times the resting latency or 300 ms more.
func canaryLimit(slowest time.Duration) time.Duration {
	return max(3*slowest, slowest+300*time.Millisecond)
}

func (c *canaryWatch) due(now time.Time) bool {
	return c.name != "" && !c.wait && !now.Before(c.next)
}

func (c *canaryWatch) sentAt(now time.Time) {
	c.wait, c.sent, c.next = true, now, now.Add(canaryEvery)
}

// answer records a state from the canary and reports whether it was too slow.
func (c *canaryWatch) answer(at time.Time) bool {
	if !c.wait || !at.After(c.sent) {
		return false
	}
	c.wait = false
	c.last = at.Sub(c.sent)
	return c.last > c.limit
}

// expire reports a read that stayed unanswered for lostAfter.
func (c *canaryWatch) expire(now time.Time) bool {
	if c.wait && now.Sub(c.sent) >= lostAfter {
		c.wait = false
		return true
	}
	return false
}
