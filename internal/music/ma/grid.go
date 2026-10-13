package ma

import (
	"cmp"
	"slices"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
)

const sameBeat = 1000

type frame struct {
	ts   int64
	down bool
}

type grid struct {
	known   []frame
	next    int
	last    int
	started bool
	tempo   float64
	prev    float64
}

func (g *grid) add(f frame) bool {
	i, _ := slices.BinarySearchFunc(g.known, f.ts, func(k frame, ts int64) int { return cmp.Compare(k.ts, ts) })
	if i > 0 && f.ts-g.known[i-1].ts < sameBeat || i < len(g.known) && g.known[i].ts-f.ts < sameBeat {
		return false
	}
	g.known = slices.Insert(g.known, i, f)
	if i < g.next {
		g.next++
	}
	g.tempo = 0
	return true
}

func (g *grid) peek() (frame, bool) {
	if g.next >= len(g.known) {
		return frame{}, false
	}
	return g.known[g.next], true
}

func (g *grid) deliver(at time.Time) music.Beat {
	f := g.known[g.next]
	var idx int
	switch {
	case f.down && !g.started:
		idx = 0
	case f.down:
		idx = floorDiv(g.last, music.PerBar)*music.PerBar + music.PerBar
	case g.started:
		idx = g.last + 1
	default:
		for k, h := range g.known[g.next:] {
			if h.down {
				idx = -k
				break
			}
		}
	}
	g.last, g.started = idx, true
	g.next++
	return music.Beat{At: at, Index: idx, InBar: (idx%music.PerBar + music.PerBar) % music.PerBar, Energy: music.UnknownEnergy}
}

func (g *grid) bpm() float64 {
	if g.tempo == 0 {
		var iv []int64
		for i := 1; i < len(g.known); i++ {
			iv = append(iv, g.known[i].ts-g.known[i-1].ts)
		}
		switch {
		case len(iv) > 0:
			slices.Sort(iv)
			g.tempo = float64(time.Minute.Microseconds()) / float64(iv[len(iv)/2])
		case g.prev > 0:
			g.tempo = g.prev
		default:
			g.tempo = music.DefaultBPM
		}
	}
	return g.tempo
}

func (g *grid) ahead() (beats, downs int) {
	for _, f := range g.known[g.next:] {
		if f.down {
			downs++
		}
	}
	return len(g.known) - g.next, downs
}

func (g *grid) clear() {
	if len(g.known) > 1 {
		g.prev = g.bpm()
	}
	*g = grid{prev: g.prev}
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && a < 0 {
		q--
	}
	return q
}
