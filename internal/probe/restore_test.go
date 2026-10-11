package probe

import (
	"testing"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

func TestRestorePayload(t *testing.T) {
	b, ct := 120, 370
	off := restorePayload(z2m.State{State: "OFF", Brightness: &b})
	if off["state"] != "OFF" || off["brightness"] != nil {
		t.Errorf("off: %v", off)
	}
	warm := restorePayload(z2m.State{State: "ON", Brightness: &b, ColorMode: "color_temp", ColorTemp: &ct, Color: &z2m.XY{X: 0.4, Y: 0.4}})
	if warm["color_temp"] != 370 || warm["color"] != nil || warm["brightness"] != 120 {
		t.Errorf("color_temp: %v", warm)
	}
	xy := restorePayload(z2m.State{State: "ON", ColorMode: "xy", Color: &z2m.XY{X: 0.6, Y: 0.37}})
	if xy["color"] == nil || xy["color_temp"] != nil {
		t.Errorf("xy: %v", xy)
	}
}
