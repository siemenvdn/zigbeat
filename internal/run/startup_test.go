package run

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

func TestStartupStopsOnCtrlC(t *testing.T) {
	r, f := newShell(t)
	f.silent = true
	r.cfg.Canary = "far-lamp"
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	if r.snapshot(ctx) {
		t.Fatal("nothing answered")
	}
	r.canaryBaseline(ctx)
	if d := time.Since(start); d > time.Second {
		t.Fatalf("startup took %s after Ctrl-C", d)
	}
}

func TestStartupRefusesGroups(t *testing.T) {
	r, f := newShell(t)
	if err := r.refuseGroups(context.Background()); err != nil {
		t.Fatalf("no groups known: %v", err)
	}
	f.groups = []string{"living-room", "lamp-2"}
	if err := r.refuseGroups(context.Background()); err == nil || !strings.Contains(err.Error(), "lamp-2") {
		t.Fatalf("lamp-2 is a group: %v", err)
	}
}

func bulb(name string, color bool) z2m.Device {
	fs := []z2m.Expose{{Name: "state", Access: 7}, {Name: "brightness", Access: 7}}
	if color {
		fs = append(fs, z2m.Expose{Name: "color_xy", Access: 7})
	}
	return z2m.Device{Name: name, Type: "Router", Supported: true, Interview: "SUCCESSFUL",
		Definition: &z2m.Definition{Source: "native", Description: "LED bulb", Exposes: []z2m.Expose{{Type: "light", Features: fs}}}}
}

func TestStartupRefusesDevices(t *testing.T) {
	r, f := newShell(t)
	if err := r.refuseDevices(context.Background()); err != nil {
		t.Fatalf("no devices known: %v", err)
	}
	f.devices = []z2m.Device{bulb("lamp-1", true), bulb("lamp-2", true), bulb("lamp-3", true), bulb("lamp-4", false)}
	if err := r.refuseDevices(context.Background()); err != nil {
		t.Fatalf("every light fits its role: %v", err)
	}
	cases := map[string]func([]z2m.Device){
		`light "lamp-2" has no xy colour`:      func(ds []z2m.Device) { ds[1] = bulb("lamp-2", false) },
		`light "lamp-4" is not a Zigbee2MQTT`:  func(ds []z2m.Device) { ds[3].Name = "lamp-9" },
		`light "lamp-3" is not offered by zig`: func(ds []z2m.Device) { ds[2].Definition.Description = "Smart plug" },
	}
	for want, mutate := range cases {
		f.devices = []z2m.Device{bulb("lamp-1", true), bulb("lamp-2", true), bulb("lamp-3", true), bulb("lamp-4", false)}
		mutate(f.devices)
		if err := r.refuseDevices(context.Background()); err == nil || !strings.HasPrefix(err.Error(), want) || !strings.HasSuffix(err.Error(), "run zigbeat setup again") {
			t.Errorf("want %s, got %v", want, err)
		}
	}
}
