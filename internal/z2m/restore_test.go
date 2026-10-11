package z2m

import "testing"

func TestRestorePayload(t *testing.T) {
	b, ct := 120, 370
	off := RestorePayload(State{State: "OFF", Brightness: &b})
	if off["state"] != "OFF" || off["brightness"] != nil {
		t.Errorf("off: %v", off)
	}
	warm := RestorePayload(State{State: "ON", Brightness: &b, ColorMode: "color_temp", ColorTemp: &ct, Color: &XY{X: 0.4, Y: 0.4}})
	if warm["color_temp"] != 370 || warm["color"] != nil || warm["brightness"] != 120 {
		t.Errorf("color_temp: %v", warm)
	}
	xy := RestorePayload(State{State: "ON", ColorMode: "xy", Color: &XY{X: 0.6, Y: 0.37}})
	if xy["color"] == nil || xy["color_temp"] != nil {
		t.Errorf("xy: %v", xy)
	}
}

func TestFrames(t *testing.T) {
	cases := []struct {
		p    map[string]any
		want int
	}{
		{map[string]any{"color": map[string]string{"hex": "#ff0000"}, "transition": 0.7}, 1},
		{map[string]any{"color": map[string]string{"hex": "#ff0000"}, "brightness": 200, "transition": 0.7}, 2},
		{map[string]any{"state": "ON", "brightness": 150}, 1},
		{map[string]any{"state": "ON", "brightness": 150, "color": map[string]string{"hex": "#ff0000"}}, 2},
		{map[string]any{"brightness_move": 40}, 1},
		{map[string]any{"brightness": 76, "brightness_move": 40}, 2},
		{map[string]any{"transition": 0}, 0},
	}
	for _, c := range cases {
		if got := Frames(c.p); got != c.want {
			t.Errorf("Frames(%v) = %d, want %d", c.p, got, c.want)
		}
	}
}
