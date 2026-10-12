package probe

import (
	"strings"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

func light(name string, color bool) z2m.Device {
	fs := []z2m.Expose{{Name: "state", Access: 7}, {Name: "brightness", Access: 7}}
	if color {
		fs = append(fs, z2m.Expose{Name: "color_xy", Access: 7})
	}
	return z2m.Device{Name: name, Type: "Router", Supported: true, Interview: "SUCCESSFUL",
		Definition: &z2m.Definition{Source: "native", Description: "LED bulb", Exposes: []z2m.Expose{{Type: "light", Features: fs}}}}
}

func TestCheckTargets(t *testing.T) {
	plug := light("plug-1", false)
	plug.Definition.Exposes = append(plug.Definition.Exposes, z2m.Expose{Type: "numeric", Name: "power"})
	devs := []z2m.Device{light("lamp-1", true), light("lamp-4", false), plug}
	cases := []struct {
		targets, mode, err string
	}{
		{"lamp-1", "color", ""},
		{"lamp-1,lamp-4", "brightness", ""},
		{"lamp-1,lamp-4", "color", `light "lamp-4" has no xy colour`},
		{"lamp-4", "combo", `light "lamp-4" has no xy colour`},
		{"plug-1", "brightness", `light "plug-1" is not offered by zigbeat setup (measures power (power)`},
	}
	for _, c := range cases {
		err := checkTargets(devs, strings.Split(c.targets, ","), c.mode)
		if c.err == "" && err != nil || c.err != "" && (err == nil || !strings.HasPrefix(err.Error(), c.err)) {
			t.Errorf("%s in %s mode: %v, want %q", c.targets, c.mode, err, c.err)
		}
	}
}
