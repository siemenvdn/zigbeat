package ma

import (
	"cmp"
	"slices"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
)

const sameBeat = 1000 // µs: a beat this close to a known one is a replay

// frame is one beat from the relay.
type frame struct {
	ts   int64 // µs on MA's clock
	down bool  // the first beat of a bar
}

// grid holds the beats of the current track and numbers them as they are delivered.
type grid struct {
	known   []frame // every beat of the track, sorted
	next    int     // the first beat not delivered yet
	last    int     // Index of the last delivered beat
	started bool    // a beat of this track was delivered
	tempo   float64 // 0 until computed for the current beats
	prev    float64 // tempo of the previous track
}

// add inserts f unless it is a replay of a known beat; it reports whether f was new.
func (g *grid) add(f frame) bool {
	i, _ := slices.BinarySearchFunc(g.known, f.ts, func(k frame, ts int64) int { return cmp.Compare(k.ts, ts) })
	if i > 0 && f.ts-g.known[i-1].ts < sameBeat || i < len(g.known) && g.known[i].ts-f.ts < sameBeat {
		return false
	}
	g.known = slices.Insert(g.known, i, f)
	if i < g.next {
		g.next++ // behind what was delivered; never delivered
	}
	g.tempo = 0
	return true
}

// peek is the next beat to deliver.
func (g *grid) peek() (frame, bool) {
	if g.next >= len(g.known) {
		return frame{}, false
	}
	return g.known[g.next], true
}

// deliver numbers the next beat, landing at at, and moves past it. A downbeat
// gets the next multiple of beat.PerBar, so moments stay on the first beat of a bar; the
// first beat of a track counts back from the first downbeat.
func (g *grid) deliver(at time.Time) beat.Beat {
	f := g.known[g.next]
	var idx int
	switch {
	case f.down && !g.started:
		idx = 0
	case f.down:
		idx = floorDiv(g.last, beat.PerBar)*beat.PerBar + beat.PerBar
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
	return beat.Beat{At: at, BPM: g.bpm(), Index: idx, InBar: (idx%beat.PerBar + beat.PerBar) % beat.PerBar, Energy: 1}
}

// bpm is the tempo of the track: a minute over the median interval of its beats.
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
			g.tempo = beat.DefaultBPM
		}
	}
	return g.tempo
}

// ahead counts the beats and downbeats not delivered yet.
func (g *grid) ahead() (beats, downs int) {
	for _, f := range g.known[g.next:] {
		if f.down {
			downs++
		}
	}
	return len(g.known) - g.next, downs
}

// clear forgets the track; its tempo stays for a next track of a single beat.
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
