package run

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/lights"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

type link interface {
	Set(light string, payload any) error
	Get(light string, payload any) error

	Await(ctx context.Context, light string, since time.Time, d time.Duration) (z2m.State, bool)

	Read(ctx context.Context, device string, d time.Duration) (time.Duration, bool)
	Done() <-chan struct{}
	Err() error

	Reconnect(ctx context.Context) bool
	Groups(ctx context.Context) ([]string, error)
	Devices(ctx context.Context) ([]z2m.Device, error)
	Close()
}

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

type restoreConn struct {
	link
	r *runner
}

func (c restoreConn) Online() bool { return c.r.online() }
