package palette

import "testing"

func TestPaletteKeysUnique(t *testing.T) {
	for _, mode := range []Mode{Color, Brightness, Combo} {
		g := New(mode)
		seen := map[int]bool{}
		n := g.Size()
		for i := 0; i < n; i++ {
			c := g.Next("a")
			if seen[c.Key] {
				t.Fatalf("%s: key %d repeats within one palette cycle", mode, c.Key)
			}
			seen[c.Key] = true
		}
		if n < 20 {
			t.Errorf("%s: palette only %d entries", mode, n)
		}
		t.Logf("%s: palette %d", mode, n)
	}
}
