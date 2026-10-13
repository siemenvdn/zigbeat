package run

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/lights"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// link is the show's connection to Zigbee2MQTT, a *lights.Session; tests
// replace it with a fake.
type link interface {
	Set(light string, payload any) error // a state for light, on its set topic
	Get(light string, payload any) error // ask light for its state
	// Await waits up to d for a state of light after since; false when none came or the connection dropped.
	Await(ctx context.Context, light string, since time.Time, d time.Duration) (z2m.State, bool)
	// Read asks device for its state and returns how long the answer took.
	Read(ctx context.Context, device string, d time.Duration) (time.Duration, bool)
	Done() <-chan struct{} // closed when the connection drops
	Err() error            // why it dropped
	// Reconnect dials until it works, and subscribes again; false when ctx is done first.
	Reconnect(ctx context.Context) bool
	Groups(ctx context.Context) ([]string, error)      // the retained group names
	Devices(ctx context.Context) ([]z2m.Device, error) // the retained device list
	Close()
}

// dialLink dials the show's session: the lights, the canary and the
// Zigbee2MQTT log, whose states and error lines go to push.
func dialLink(cfg config.Config, push func(event)) (*lights.Session, error) {
	devices := cfg.Lights()
	if cfg.Canary != "" {
		devices = append(devices, cfg.Canary)
	}
	return lights.Dial(cfg.MQTT, "run", lights.Watch{
		Devices: devices,
		OnState: func(d string, st z2m.State, at time.Time) { push(event{light: d, st: st, at: at}) },
		OnLog: func(e z2m.LogEntry, at time.Time) {
			if e.Level == "error" {
				push(event{msg: e.Message, at: at})
			}
		},
	})
}

// restoreConn is the link for the snapshot and the restore: Online dials a lost broker again for a while.
type restoreConn struct {
	link
	r *runner
}

func (c restoreConn) Online() bool { return c.r.online() }
