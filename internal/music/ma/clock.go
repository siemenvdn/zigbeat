package ma

import "time"

const keepBursts = 3 // bursts of time answers the estimate looks back on

// estimate is one time answer: MA's clock minus zigbeat's, and the round trip, in µs.
type estimate struct{ offset, rtt int64 }

// clock maps MA's monotonic clock onto zigbeat's. zigbeat's clock is µs since base.
type clock struct {
	base   time.Time
	bursts []estimate // the best answer of each recent burst, newest last; rtt -1 while empty
}

func newClock() *clock { return &clock{base: time.Now()} }

// now is zigbeat's clock in µs.
func (c *clock) now() int64 { return time.Since(c.base).Microseconds() }

// burst starts a new burst of answers; the oldest beyond keepBursts is forgotten.
func (c *clock) burst() {
	if n := len(c.bursts); n > 0 && c.bursts[n-1].rtt < 0 {
		return // the last burst got no answer: keep the older ones rather than lose the estimate
	}
	c.bursts = append(c.bursts, estimate{rtt: -1})
	if len(c.bursts) > keepBursts {
		c.bursts = c.bursts[1:]
	}
}

// add records an answer: t1 and t4 are zigbeat's send and receive times, t2 and t3 MA's.
func (c *clock) add(t1, t2, t3, t4 int64) {
	if len(c.bursts) == 0 {
		c.burst()
	}
	e := estimate{offset: ((t2 - t1) + (t3 - t4)) / 2, rtt: (t4 - t1) - (t3 - t2)}
	if b := &c.bursts[len(c.bursts)-1]; b.rtt < 0 || e.rtt < b.rtt {
		*b = e
	}
}

// best is the answer with the shortest round trip; false before the first answer.
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

// at is the local time of ts on MA's clock; ok is false before the first answer.
func (c *clock) at(ts int64) (time.Time, bool) {
	e, ok := c.best()
	return c.base.Add(time.Duration(ts-e.offset) * time.Microsecond), ok
}
