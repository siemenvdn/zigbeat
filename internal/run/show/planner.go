// Package show plans the light show.
package show

import (
	"fmt"
	"math"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Message is one message the planner wants to land at a moment.
type Message struct {
	Light   string
	Payload map[string]any
	Land    time.Time
	Drop    bool
	before  string
}

type stroke struct {
	at     time.Time
	fall   time.Duration
	blend  float64
	kick   bool
	moment int
}

// Planner decides per moment which lights change and how.
type Planner struct {
	pace    pace
	palette palette
	colors  colorShow
	breathe breatheShow
	energy  energy
	notes   []string
	moments int
}

func NewPlanner(c config.Config) *Planner {
	p := &Planner{
		pace:    pace{bpm: music.DefaultBPM, rate: budgetRate(c)},
		palette: palette{colors: c.Effects.Palette},
	}
	p.colors = newColorShow(c.Fixtures, &p.palette)
	p.breathe = newBreatheShow(c.Fixtures)
	p.SetEffects(c.Effects)
	return p
}

func level(percent int) int {
	return int(math.Round(float64(percent) * z2m.MaxBrightness / 100))
}

// Start turns every light on at its starting level, landing at now.
func (p *Planner) Start(now time.Time) []Message {
	return append(p.colors.start(now), p.breathe.start(now)...)
}

// Beat returns the messages for b, or nothing when b is not a moment.
func (p *Planner) Beat(b music.Beat, calm bool) []Message {
	p.breathe.heard(b.At, p.pace.period())
	if p.energy.hear(b) {
		p.note("energy %s from bar %d: every %s", p.energy, b.Bar(), music.Span(p.pace.now(p.energy.quiet)))
	}
	n := p.interval()
	if b.Index%n != 0 {
		return nil
	}
	if s := p.pace.noteStretch(n, p.energy.quiet); s != "" {
		p.note("%s", s)
	}
	p.rotate(b)
	calm = calm || p.energy.quiet
	room := p.pace.room(n)
	st := stroke{at: b.At, fall: p.pace.fall(n), blend: p.pace.blend(n), moment: p.moments}
	out := p.colors.plan(st, calm, room)
	out = append(out, p.breathe.plan(st, calm, room-float64(framesOf(out)))...)
	p.moments++
	return out
}

func (p *Planner) interval() int {
	return p.pace.interval(p.energy.quiet, float64(p.colors.minFrames()))
}

func (p *Planner) rotate(b music.Beat) {
	v, w := p.colors.variantAt(b.Index), p.breathe.at(b.Index)
	if v == p.colors.playing && w == p.breathe.playing {
		return
	}
	p.colors.play(v)
	p.breathe.playing = w
	note := "variant " + string(v)
	if w != "" {
		note += ", breathe " + string(w)
	}
	p.note("%s from bar %d", note, b.Bar())
}

// Track takes the properties of the track that plays now.
func (p *Planner) Track(t music.Track) {
	p.pace.bpm, p.palette.art, p.energy.lowPart = t.BPM, t.Palette, false
	p.choose()
}

// SetEffects takes the style of the show from the next moment on.
func (p *Planner) SetEffects(e config.Effects) {
	p.palette.fallback, p.palette.album, p.palette.n = e.Palette, e.AlbumPalette, e.AlbumColors
	p.pace.beats, p.energy.follow = e.Pace, e.FollowEnergy
	p.colors.style(e)
	p.breathe.style(e)
	p.choose()
}

func (p *Planner) choose() {
	if note := p.palette.choose(); note != "" {
		p.note("%s", note)
	}
}

// Settle gives every breathe light its low end once per gap in the beats.
func (p *Planner) Settle(now time.Time) []Message { return p.breathe.settle(now) }

// Missed takes back the colours of messages the engine dropped, given in queue order, newest first.
func (p *Planner) Missed(ms ...Message) { p.colors.missed(ms) }

// Moments counts the moments planned so far.
func (p *Planner) Moments() int { return p.moments }

func (p *Planner) note(format string, args ...any) {
	p.notes = append(p.notes, fmt.Sprintf(format, args...))
}

// TakeNotes returns what the operator should know since the last call.
func (p *Planner) TakeNotes() []string {
	n := p.notes
	p.notes = nil
	return n
}

func framesOf(ms []Message) int {
	n := 0
	for _, m := range ms {
		n += z2m.Frames(m.Payload)
	}
	return n
}

func split(u []string) [][]string {
	var ps [][]string
	for len(u) > flow.MaxPerMoment {
		ps = append(ps, u[:flow.MaxPerMoment])
		u = u[flow.MaxPerMoment:]
	}
	return append(ps, u)
}
