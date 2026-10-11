// Package palette makes probe commands whose echoes can be told apart.
package palette

import "github.com/siemenvdn/zigbeat/internal/z2m"

// Command is a set payload and the key its echo is matched by.
type Command struct {
	Key     int
	Payload map[string]any
}

// Generator hands out commands whose echo can be told apart: no key repeats
// for a device until the whole palette has been used.
type Generator struct {
	mode   string
	colors []z2m.XY
	levels []int
	next   map[string]int
}

// New returns a generator for mode: color, brightness or combo.
func New(mode string) *Generator {
	g := &Generator{mode: mode, next: map[string]int{}}
	seen := map[int]bool{}
	for i := 0; i < 72 && len(g.colors) < 36; i++ {
		xy := hueToXY(float64(i) * 5)
		k := xKey(xy.X)
		if seen[k] || seen[k-1] || seen[k+1] {
			continue
		}
		seen[k] = true
		g.colors = append(g.colors, xy)
	}
	for b := 40; b <= 250; b += 6 {
		g.levels = append(g.levels, b)
	}
	return g
}

// Size is the number of commands before a key repeats.
func (g *Generator) Size() int {
	if g.mode == "brightness" {
		return len(g.levels)
	}
	return len(g.colors)
}

// Next returns the next command for dev.
func (g *Generator) Next(dev string) Command {
	i := g.next[dev]
	g.next[dev] = i + 1
	switch g.mode {
	case "brightness":
		b := g.levels[i%len(g.levels)]
		return Command{Key: b, Payload: map[string]any{"brightness": b, "transition": 0}}
	case "combo":
		c := g.colors[i%len(g.colors)]
		b := 140
		if i%2 == 0 {
			b = 254
		}
		return Command{Key: xKey(c.X), Payload: map[string]any{"color": c, "brightness": b, "transition": 0}}
	default:
		c := g.colors[i%len(g.colors)]
		return Command{Key: xKey(c.X), Payload: map[string]any{"color": c, "transition": 0}}
	}
}
