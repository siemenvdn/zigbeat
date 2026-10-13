package ma

import "time"

const keepBursts = 3

type estimate struct{ offset, rtt int64 }

type clock struct {
	base   time.Time
	bursts []estimate
}

func newClock() *clock { return &clock{base: time.Now()} }

func (c *clock) now() int64 { return time.Since(c.base).Microseconds() }

func (c *clock) burst() {
	if n := len(c.bursts); n > 0 && c.bursts[n-1].rtt < 0 {
		return
	}
	c.bursts = append(c.bursts, estimate{rtt: -1})
	if len(c.bursts) > keepBursts {
		c.bursts = c.bursts[1:]
	}
}

func (c *clock) add(t1, t2, t3, t4 int64) {
	if len(c.bursts) == 0 {
		c.burst()
	}
	e := estimate{offset: ((t2 - t1) + (t3 - t4)) / 2, rtt: (t4 - t1) - (t3 - t2)}
	if b := &c.bursts[len(c.bursts)-1]; b.rtt < 0 || e.rtt < b.rtt {
		*b = e
	}
}

func (c *clock) best() (estimate, bool) {
	var best estimate
	ok := false
	for _, b := range c.bursts {
		if b.rtt >= 0 && (!ok || b.rtt < best.rtt) {
			best, ok = b, true
		}
	}
	return best, ok
}

func (c *clock) at(ts int64) (time.Time, bool) {
	e, ok := c.best()
	return c.base.Add(time.Duration(ts-e.offset) * time.Microsecond), ok
}
