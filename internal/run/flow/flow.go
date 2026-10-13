// Package flow keeps the show within what the network can take.
package flow

const (
	MaxPerMoment  = 3
	kickFrames    = 2
	breatheFrames = 1

	MaxInFlight = 2 * MaxPerMoment

	Burst = MaxPerMoment * (kickFrames + breatheFrames)
)
