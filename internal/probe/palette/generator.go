// Package palette makes probe commands whose echoes can be told apart.
package palette

import "github.com/siemenvdn/zigbeat/internal/z2m"

const (
	hueStep   = 5
	maxColors = 36

	KeySlack = 1

	minLevel, maxLevel, levelStep = 40, 250, 6

	comboLow = 140
)

// Mode is the kind of command the probe sends.
type Mode string

// The modes.
const (
	Color      Mode = "color"
	Brightness Mode = "brightness"
	Combo      Mode = "combo"
)

// Command is a set payload and the key its echo is matched by.
type Command struct {
	Key     int
	Payload map[string]any
}

// Generator hands out commands whose echo can be told apart.
type Generator struct {
	mode   Mode
	colors []z2m.XY
	levels []int
	next   map[string]int
}

// New returns a generator for mode.
func New(mode Mode) *Generator {
	g := &Generator{mode: mode, next: map[string]int{}}
	seen := map[int]bool{}
	taken := func(k int) bool {
		for d := -KeySlack; d <= KeySlack; d++ {
			if seen[k+d] {
				return true
			}
		}
		return false
	}
	for h := 0; h < 360 && len(g.colors) < maxColors; h += hueStep {
		xy := hueToXY(float64(h))
		k := xKey(xy.X)
		if taken(k) {
			continue
		}
		seen[k] = true
		g.colors = append(g.colors, xy)
	}
	for b := minLevel; b <= maxLevel; b += levelStep {
		g.levels = append(g.levels, b)
	}
	return g
}

// Size is the number of commands before a key repeats.
func (g *Generator) Size() int {
	if g.mode == Brightness {
		return len(g.levels)
	}
	return len(g.colors)
}

// Next returns the next command for dev.
func (g *Generator) Next(dev string) Command {
	i := g.next[dev]
	g.next[dev] = i + 1
	switch g.mode {
	case Brightness:
		b := g.levels[i%len(g.levels)]
		return Command{Key: b, Payload: map[string]any{"brightness": b, "transition": 0}}
	case Combo:
		c := g.colors[i%len(g.colors)]
		b := comboLow
		if i%2 == 0 {
			b = z2m.MaxBrightness
		}
		return Command{Key: xKey(c.X), Payload: map[string]any{"color": c, "brightness": b, "transition": 0}}
	default:
		c := g.colors[i%len(g.colors)]
		return Command{Key: xKey(c.X), Payload: map[string]any{"color": c, "transition": 0}}
	}
}
