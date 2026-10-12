package flow

import "time"

const (
	stepEvery    = 10 * time.Second // at most one step down per this
	RecoverAfter = 60 * time.Second // quiet time before one step back up

	// The levels of the ladder above 0, normal.
	CalmLevel   = 1 // no kick, one message in flight
	SparseLevel = 2 // half as many moments
	PauseLevel  = 3 // nothing but canary reads
)

// Backoff is the ladder from 0 to PauseLevel. It never stops the show.
type Backoff struct {
	level    int
	lastDown time.Time
	lastUp   time.Time
	quiet    time.Time // last signal
}

// Signal raises the level by one, at most once per stepEvery and never above max.
// It returns true when the level changed. A signal capped below the current
// level does not count, so it cannot hold a higher level.
func (b *Backoff) Signal(now time.Time, max int) bool {
	if b.level > max {
		return false
	}
	b.quiet = now
	if b.level >= max || now.Sub(b.lastDown) < stepEvery {
		return false
	}
	b.level++
	b.lastDown = now
	return true
}

// Pause jumps straight to the pause level.
func (b *Backoff) Pause(now time.Time) {
	b.level, b.lastDown, b.quiet = PauseLevel, now, now
}

// Tick lowers the level by one after RecoverAfter without signals. It returns true when it did.
func (b *Backoff) Tick(now time.Time) bool {
	if b.level == 0 || now.Sub(b.quiet) < RecoverAfter || now.Sub(b.lastUp) < RecoverAfter {
		return false
	}
	b.level--
	b.lastUp = now
	return true
}

// Level is the current step: 0 normal up to PauseLevel.
func (b *Backoff) Level() int { return b.level }
