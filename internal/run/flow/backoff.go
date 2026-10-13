package flow

import "time"

const (
	stepEvery    = 10 * time.Second
	RecoverAfter = 60 * time.Second

	CalmLevel  = 1
	PauseLevel = 2
)

// Backoff is the ladder from 0 to PauseLevel.
type Backoff struct {
	level    int
	lastDown time.Time
	lastUp   time.Time
	quiet    time.Time
}

// Signal raises the level by one, at most once per stepEvery and never above max.
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

// Tick lowers the level by one after RecoverAfter without signals.
func (b *Backoff) Tick(now time.Time) bool {
	if b.level == 0 || now.Sub(b.quiet) < RecoverAfter || now.Sub(b.lastUp) < RecoverAfter {
		return false
	}
	b.level--
	b.lastUp = now
	return true
}

// Level is the current step.
func (b *Backoff) Level() int { return b.level }
