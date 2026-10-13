package setup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

type fakeBridge struct {
	devs   []z2m.Device
	silent map[string]bool
	reads  []string
	stopAt int
	stop   func()
}

func (f *fakeBridge) Devices(context.Context) ([]z2m.Device, error) {
	if f.devs == nil {
		return nil, fmt.Errorf("%w zigbee2mqtt/bridge/devices within %v", z2m.ErrNoRetained, z2m.RetainedWait)
	}
	return f.devs, nil
}

func (f *fakeBridge) Read(_ context.Context, name string, _ time.Duration) (time.Duration, bool) {
	f.reads = append(f.reads, name)
	if len(f.reads) == f.stopAt {
		f.stop()
		return 0, false
	}
	return 40 * time.Millisecond, !f.silent[name]
}

func dev(name, kind string) z2m.Device {
	fs := []z2m.Expose{{Name: "state", Access: 7}, {Name: "brightness", Access: 7}}
	exposes := []z2m.Expose{{Type: "light", Features: fs}}
	switch kind {
	case "color":
		exposes[0].Features = append(fs, z2m.Expose{Name: "color_xy", Access: 7})
	case "plug":
		exposes = []z2m.Expose{{Type: "switch"}, {Type: "numeric", Name: "power"}}
	case "sensor":
		exposes = []z2m.Expose{{Type: "numeric", Name: "temperature"}}
	}
	typ := "Router"
	if kind == "sensor" {
		typ = "EndDevice"
	}
	return z2m.Device{Name: name, Type: typ, Supported: true, Interview: "SUCCESSFUL",
		Definition: &z2m.Definition{Source: "native", Vendor: "Acme", Model: kind, Description: "test " + kind, Exposes: exposes}}
}

func home() []z2m.Device {
	return []z2m.Device{
		{Name: "Coordinator", Type: "Coordinator"},
		dev("pendant-1", "color"), dev("pendant-2", "color"), dev("pendant-3", "color"), dev("floor-lamp", "white"),
		dev("desk-lamp", "color"), dev("plug-1", "plug"), dev("far-lamp", "color"), dev("sensor-1", "sensor"),
	}
}

var today = time.Date(2026, 10, 12, 21, 0, 0, 0, time.UTC)

func start(t *testing.T, br *fakeBridge, path, existing, stdin string, flags ...string) (int, string) {
	t.Helper()
	t.Setenv("ZIGBEAT_MQTT_PASSWORD", "secret")
	if existing != "" {
		if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	o, err := parseFlags(append([]string{"-config", path}, flags...))
	if err == nil {
		err = o.load()
	}
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	br.stop = cancel
	s := &session{o: o, br: br, in: newInput(strings.NewReader(stdin)), out: &out, err: &out, today: today}
	return s.run(ctx), out.String()
}
