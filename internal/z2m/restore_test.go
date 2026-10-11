package z2m

import "testing"

func TestRestorePayloads(t *testing.T) {
	b, ct := 120, 370
	off := RestorePayloads(State{State: "OFF", Brightness: &b, ColorMode: "xy", Color: &XY{X: 0.6, Y: 0.37}})
	if len(off) != 2 || off[0]["state"] != "ON" || off[0]["brightness"] != 120 || off[0]["color"] == nil || off[1]["state"] != "OFF" {
		t.Errorf("a light that was off gets its level and colour back, then goes off: %v", off)
	}
	bare := RestorePayloads(State{State: "OFF"})
	if len(bare) != 1 || bare[0]["state"] != "OFF" {
		t.Errorf("off without a saved level: %v", bare)
	}
	warm := RestorePayloads(State{State: "ON", Brightness: &b, ColorMode: "color_temp", ColorTemp: &ct, Color: &XY{X: 0.4, Y: 0.4}})
	if len(warm) != 1 || warm[0]["color_temp"] != 370 || warm[0]["color"] != nil || warm[0]["brightness"] != 120 {
		t.Errorf("color_temp: %v", warm)
	}
	xy := RestorePayloads(State{State: "ON", ColorMode: "xy", Color: &XY{X: 0.6, Y: 0.37}})
	if len(xy) != 1 || xy[0]["color"] == nil || xy[0]["color_temp"] != nil {
		t.Errorf("xy: %v", xy)
	}
}
