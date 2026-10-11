// Package flow keeps the show within what the network can take: the frame
// budget, one message in flight per light, the backoff ladder and the canary.
package flow

import (
	"math"
	"time"
)

// Burst fits one moment: 3 messages of 2 frames.
const Burst = 6

// Limiter is a token bucket in Zigbee frames; the rate is a ceiling, never raised.
type Limiter struct {
	rate   float64
	tokens float64
	last   time.Time
}

func NewLimiter(rate float64, now time.Time) *Limiter {
	return &Limiter{rate: rate, tokens: Burst, last: now}
}

// Take spends n frames if the bucket holds them.
func (l *Limiter) Take(now time.Time, n int) bool {
	if now.After(l.last) {
		l.tokens = math.Min(Burst, l.tokens+now.Sub(l.last).Seconds()*l.rate)
		l.last = now
	}
	if l.tokens < float64(n) {
		return false
	}
	l.tokens -= float64(n)
	return true
}
