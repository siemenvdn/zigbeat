package flow

import "time"

const (
	stepEvery    = 10 * time.Second // at most one step down per this
	recoverAfter = 60 * time.Second // quiet time before one step back up
	PauseLevel   = 3
)

// Backoff is the ladder: 0 normal, 1 no kick and one message in flight,
// 2 half as many moments, 3 pause. It never stops the show.
type Backoff struct {
	level    int
	lastDown time.Time
	lastUp   time.Time
	quiet    time.Time // last signal
	named    []time.Time
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

// NamedError records an error about one of our lights and reports whether
// three of them fell within a minute.
func (b *Backoff) NamedError(now time.Time) bool {
	keep := b.named[:0]
	for _, t := range b.named {
		if now.Sub(t) < time.Minute {
			keep = append(keep, t)
		}
	}
	b.named = append(keep, now)
	return len(b.named) >= 3
}

// Tick lowers the level by one after recoverAfter without signals. It returns true when it did.
func (b *Backoff) Tick(now time.Time) bool {
	if b.level == 0 || now.Sub(b.quiet) < recoverAfter || now.Sub(b.lastUp) < recoverAfter {
		return false
	}
	b.level--
	b.lastUp = now
	return true
}

// Level is the current step: 0 normal up to PauseLevel.
func (b *Backoff) Level() int { return b.level }
