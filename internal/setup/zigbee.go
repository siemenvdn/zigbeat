package setup

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

type zigbee interface {
	Devices(ctx context.Context) ([]z2m.Device, error)

	Read(ctx context.Context, device string, d time.Duration) (time.Duration, bool)
}
