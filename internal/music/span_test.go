package music

import "testing"

func TestSpan(t *testing.T) {
	for beats, want := range map[int]string{1: "beat", 2: "2 beats", 3: "3 beats", 4: "bar", 6: "6 beats", 8: "2 bars", 16: "4 bars"} {
		if got := Span(beats); got != want {
			t.Errorf("Span(%d) = %q, want %q", beats, got, want)
		}
	}
}
