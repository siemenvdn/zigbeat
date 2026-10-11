package run

import "time"

const (
	stepEvery    = 10 * time.Second // at most one step down per this
	recoverAfter = 60 * time.Second // quiet time before one step back up
	pauseLevel   = 3
)

// backoff is the ladder: 0 normal, 1 no kick and one message in flight,
// 2 half as many moments, 3 pause. It never stops the show.
type backoff struct {
	level    int
	lastDown time.Time
	lastUp   time.Time
	quiet    time.Time // last signal
	named    []time.Time
}

// signal raises the level by one, at most once per stepEvery and never above max.
// It returns true when the level changed. A signal capped below the current
// level does not count, so it cannot hold a higher level.
func (b *backoff) signal(now time.Time, max int) bool {
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

// pause jumps straight to the pause level.
func (b *backoff) pause(now time.Time) {
	b.level, b.lastDown, b.quiet = pauseLevel, now, now
}

// namedError records an error about one of our lights and reports whether
// three of them fell within a minute.
func (b *backoff) namedError(now time.Time) bool {
	keep := b.named[:0]
	for _, t := range b.named {
		if now.Sub(t) < time.Minute {
			keep = append(keep, t)
		}
	}
	b.named = append(keep, now)
	return len(b.named) >= 3
}

// tick lowers the level by one after recoverAfter without signals. It returns true when it did.
func (b *backoff) tick(now time.Time) bool {
	if b.level == 0 || now.Sub(b.quiet) < recoverAfter || now.Sub(b.lastUp) < recoverAfter {
		return false
	}
	b.level--
	b.lastUp = now
	return true
}
