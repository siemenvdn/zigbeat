package setup

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// zigbee is what setup needs from Zigbee2MQTT, a *lights.Session; tests replace it with a fake.
type zigbee interface {
	// Devices is Zigbee2MQTT's retained device list.
	Devices(ctx context.Context) ([]z2m.Device, error)
	// Read asks device for its state and returns how long the answer took, waiting up to d.
	Read(ctx context.Context, device string, d time.Duration) (time.Duration, bool)
}
