// Package lights saves the state of lights before zigbeat touches them and puts
// it back afterwards, one light at a time within the budget.
package lights

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Conn is what saving and restoring needs from the connection to Zigbee2MQTT.
type Conn interface {
	// Get asks light for its state, on <base_topic>/<light>/get.
	Get(light string, payload any) error
	// Set sends light a state, on <base_topic>/<light>/set.
	Set(light string, payload any) error
	// Await waits up to d for a state from light that arrived after since. It is
	// false when none came, the connection dropped or ctx is done.
	Await(ctx context.Context, light string, since time.Time, d time.Duration) (z2m.State, bool)
	// Online reports whether the broker is there, dialling again first when it can.
	Online() bool
}

// answerWait is how long a read or a set waits for the state: until it counts as
// lost, so a slow light never gets a second command before the first one is confirmed.
const answerWait = z2m.LostAfter

// pace is the time frames take at rate frames per second.
func pace(frames int, rate float64) time.Duration {
	return time.Duration(float64(frames) / rate * float64(time.Second))
}
