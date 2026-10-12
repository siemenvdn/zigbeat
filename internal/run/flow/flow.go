// Package flow keeps the show within what the network can take: the frame
// budget, one message in flight per light, the backoff ladder and the canary.
package flow

const (
	// MaxPerMoment is the most colour messages one moment sends, and the most white ones.
	MaxPerMoment = 3
	kickFrames   = 2 // a kick sets brightness and colour
	whiteFrames  = 1 // a pulse or a flip sets brightness
	// MaxInFlight is one full moment in flight: its colour and its white messages.
	MaxInFlight = 2 * MaxPerMoment
	// Burst fits one full moment: MaxPerMoment kicks and MaxPerMoment white messages.
	Burst = MaxPerMoment * (kickFrames + whiteFrames)
)
