// Package flow keeps the show within what the network can take: the frame
// budget, one message in flight per light, the backoff ladder and the canary.
package flow

const (
	// MaxPerMoment is the most messages one moment sends.
	MaxPerMoment = 3
	kickFrames   = 2 // a kick sets brightness and colour
	// Burst fits one moment: MaxPerMoment messages of kickFrames.
	Burst = MaxPerMoment * kickFrames
)
