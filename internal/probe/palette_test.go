package probe

import "testing"

func TestPaletteKeysUnique(t *testing.T) {
	for _, mode := range []string{"color", "brightness", "combo"} {
		g := newGenerator(mode)
		seen := map[int]bool{}
		n := g.paletteSize()
		for i := 0; i < n; i++ {
			c := g.nextFor("a")
			if seen[c.key] {
				t.Fatalf("%s: key %d repeats within one palette cycle", mode, c.key)
			}
			seen[c.key] = true
		}
		if n < 20 {
			t.Errorf("%s: palette only %d entries", mode, n)
		}
		t.Logf("%s: palette %d", mode, n)
	}
}
