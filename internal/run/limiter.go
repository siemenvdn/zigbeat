package run

import (
	"math"
	"time"
)

// burst fits one moment: 3 messages of 2 frames.
const burst = 6

// limiter is a token bucket in Zigbee frames; the rate is a ceiling, never raised.
type limiter struct {
	rate   float64
	tokens float64
	last   time.Time
}

func newLimiter(rate float64, now time.Time) *limiter {
	return &limiter{rate: rate, tokens: burst, last: now}
}

// take spends n frames if the bucket holds them.
func (l *limiter) take(now time.Time, n int) bool {
	if now.After(l.last) {
		l.tokens = math.Min(burst, l.tokens+now.Sub(l.last).Seconds()*l.rate)
		l.last = now
	}
	if l.tokens < float64(n) {
		return false
	}
	l.tokens -= float64(n)
	return true
}
