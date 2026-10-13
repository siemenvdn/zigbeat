// Package lights saves the state of lights before zigbeat touches them and puts it back afterwards, one light
// at a time within the budget.
package lights

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Conn is what saving and restoring needs from the connection to Zigbee2MQTT.
type Conn interface {
	Get(light string, payload any) error

	Set(light string, payload any) error

	Await(ctx context.Context, light string, since time.Time, d time.Duration) (z2m.State, bool)

	Online() bool
}

const answerWait = z2m.LostAfter

func pace(frames int, rate float64) time.Duration {
	return time.Duration(float64(frames) / rate * float64(time.Second))
}
