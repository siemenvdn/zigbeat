package z2m

import "testing"

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

func TestEchoes(t *testing.T) {
	if !Echoes(map[string]any{"color": map[string]string{"hex": "#ff0000"}, "transition": 0.7}) {
		t.Error("a colour change is answered with a state")
	}
	if Echoes(map[string]any{"brightness_move": 40}) {
		t.Error("a brightness_move alone gets no state")
	}
}
