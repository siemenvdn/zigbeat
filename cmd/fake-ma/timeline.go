package main

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
)

const (
	noiseSeed  = 1                      // the hi-hats sound the same every run
	firstAt    = 250 * time.Millisecond // media time of a track's first beat
	startDelay = 2 * time.Second        // a client can connect before the first track
	seekJump   = 10 * time.Second       // far enough to show in the beat numbering
)

// timeline plays the tracks on the fake's clock and fans every frame out to the viewers.
type timeline struct {
	t0      time.Time
	offset  int64 // µs added to the monotonic clock
	log     bool
	colors  bool          // a palette per track, as MA with its color_tint option on
	art     time.Duration // the palette comes this long after a track starts
	mu      sync.Mutex
	playing bool
	beats   [][]byte // the published beat frames of the current track
	viewers map[*viewer]bool
	// lastColor is the last colour message, sent again to a viewer that joins.
	lastColor []byte
	// The music of the current track: its media time zero on the fake's clock,
	// and the first sample of the next waveform frame.
	cur    track
	pickup int
	anchor int64
	next   int64
	noise  *rand.Rand
}

func newTimeline(offset time.Duration, log, colors bool, art time.Duration) *timeline {
	return &timeline{t0: time.Now(), offset: offset.Microseconds(), log: log, colors: colors, art: art, viewers: map[*viewer]bool{},
		noise: rand.New(rand.NewPCG(noiseSeed, noiseSeed))}
}

// now is the fake's clock in µs, like MA's time.monotonic().
func (tl *timeline) now() int64 { return time.Since(tl.t0).Microseconds() + tl.offset }

// wall is the local time of ts on the fake's clock.
func (tl *timeline) wall(ts int64) time.Time {
	return tl.t0.Add(time.Duration(ts-tl.offset) * time.Microsecond)
}

func (tl *timeline) sleepUntil(ts int64) { time.Sleep(time.Until(tl.wall(ts))) }

func (tl *timeline) play(tracks []track, loop bool, analysis time.Duration, pickup int, seek, gap time.Duration) {
	go tl.waveform()
	time.Sleep(startDelay)
	for round := 0; round == 0 || loop; round++ {
		for i, tr := range tracks {
			first := round == 0
			anchor := tl.now()
			end := anchor + tr.length.Microseconds()
			if tl.colors {
				tl.cover(palettes[(round*len(tracks)+i)%len(palettes)], tl.art)
			}
			tl.music(tr, pickup, anchor)
			tl.clear(true, fmt.Sprintf("track %d, %g BPM", i+1, tr.bpm))
			if first {
				tl.sleepUntil(anchor + analysis.Microseconds())
			}
			tl.publish(tr, anchor, pickup)
			if first && seek > 0 && seek < tr.length-seekJump {
				tl.sleepUntil(anchor + seek.Microseconds())
				anchor -= seekJump.Microseconds()
				end -= seekJump.Microseconds()
				tl.music(tr, pickup, anchor)
				tl.clear(true, "seek "+cli.Seconds(seekJump)+" ahead")
				tl.publish(tr, anchor, pickup)
			}
			tl.sleepUntil(end)
			if gap > 0 {
				tl.end()
				time.Sleep(gap)
			}
		}
	}
	tl.end()
}

// clear starts a new timeline, like MA re-anchoring: the old beats no longer apply.
func (tl *timeline) clear(playing bool, why string) {
	fmt.Printf("%s  %s\n", time.Now().Format(cli.ClockMS), why)
	tl.mu.Lock()
	tl.playing, tl.beats = playing, nil
	tl.mu.Unlock()
	tl.fan([]byte(`{"type": "stream/clear"}`), true)
}

// end stops the music, like MA's stream/end; a second end is a no-op.
func (tl *timeline) end() {
	tl.mu.Lock()
	was := tl.playing
	tl.playing, tl.beats = false, nil
	tl.mu.Unlock()
	if !was {
		return
	}
	fmt.Printf("%s  stopped\n", time.Now().Format(cli.ClockMS))
	tl.fan([]byte(`{"type": "stream/end"}`), true)
}

// join adds a viewer and gives it the beats still ahead and the last colour, as MA's relay does,
// under one lock so none is missed.
func (tl *timeline) join(v *viewer) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	now := tl.now()
	for _, f := range tl.beats {
		if frameTime(f) > now {
			v.out <- outMsg{false, f}
		}
	}
	if tl.lastColor != nil {
		v.out <- outMsg{true, tl.lastColor}
	}
	tl.viewers[v] = true
}
