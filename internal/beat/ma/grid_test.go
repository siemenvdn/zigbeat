package ma

import (
	"slices"
	"testing"
	"time"
)

// track returns n beats 500 ms apart, with downbeats at the given beat numbers.
func track(n int, downs ...int) []frame {
	fs := make([]frame, n)
	for i := range fs {
		fs[i] = frame{ts: int64(i) * 500_000, down: slices.Contains(downs, i)}
	}
	return fs
}

func indices(g *grid) []int {
	var out []int
	for {
		if _, ok := g.peek(); !ok {
			return out
		}
		out = append(out, g.deliver(time.Time{}).Index)
	}
}

func TestIndex(t *testing.T) {
	cases := []struct {
		name  string
		beats []frame
		want  []int
	}{
		{"4/4", track(8, 0, 4), []int{0, 1, 2, 3, 4, 5, 6, 7}},
		{"pickup of 2", track(7, 2, 6), []int{-2, -1, 0, 1, 2, 3, 4}},
		{"bar of 3", track(8, 0, 3, 7), []int{0, 1, 2, 4, 5, 6, 7, 8}},
		{"bar of 5", track(6, 0, 5), []int{0, 1, 2, 3, 4, 8}},
		{"no downbeats", track(4), []int{0, 1, 2, 3}},
	}
	for _, c := range cases {
		g := &grid{}
		for _, f := range c.beats {
			g.add(f)
		}
		if got := indices(g); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestInBarAndTempo(t *testing.T) {
	g := &grid{}
	for _, f := range track(9, 2, 6) {
		g.add(f)
	}
	g.add(frame{ts: 9*500_000 + 400_000}) // one odd interval does not move the tempo
	if b := g.deliver(time.Time{}); b.Index != -2 || b.InBar != 2 || b.BPM != 120 || b.Energy != 1 {
		t.Errorf("first beat %+v", b)
	}
	if beats, downs := g.ahead(); beats != 9 || downs != 2 {
		t.Errorf("ahead %d beats, %d downbeats", beats, downs)
	}
	g.clear()
	g.add(frame{ts: 1})
	if b := g.deliver(time.Time{}); b.BPM != 120 || b.Index != 0 {
		t.Errorf("a single beat keeps the previous tempo and counts from 0: %+v", b)
	}
}

func TestReplays(t *testing.T) {
	g := &grid{}
	for _, f := range track(4, 0) {
		g.add(f)
	}
	g.deliver(time.Time{})
	if g.add(frame{ts: 500_000 + 800}) {
		t.Error("a beat within 1 ms of a known one is a replay")
	}
	if !g.add(frame{ts: -500_000}) {
		t.Error("an earlier beat is new")
	}
	if f, _ := g.peek(); f.ts != 500_000 {
		t.Errorf("a beat behind the delivered ones is never delivered; next %+v", f)
	}
}
