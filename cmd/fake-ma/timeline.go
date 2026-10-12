package main

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

const (
	beatTag  = 17
	waveTag  = 22
	waveLead = 5 * time.Second
	firstAt  = 250 * time.Millisecond // media time of a track's first beat
)

// timeline plays the tracks on the fake's clock and fans every frame out to the viewers.
type timeline struct {
	t0      time.Time
	offset  int64 // µs added to the monotonic clock
	log     bool
	mu      sync.Mutex
	playing bool
	beats   [][]byte // the published beat frames of the current track
	viewers map[*viewer]bool
}

func newTimeline(offset time.Duration, log bool) *timeline {
	return &timeline{t0: time.Now(), offset: offset.Microseconds(), log: log, viewers: map[*viewer]bool{}}
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
	time.Sleep(2 * time.Second)
	for round := 0; round == 0 || loop; round++ {
		for i, tr := range tracks {
			first := round == 0
			anchor := tl.now()
			end := anchor + tr.length.Microseconds()
			tl.clear(true, fmt.Sprintf("track %d, %g BPM", i+1, tr.bpm))
			if first {
				tl.sleepUntil(anchor + analysis.Microseconds())
			}
			tl.publish(tr, anchor, pickup)
			if first && seek > 0 && seek < tr.length-10*time.Second {
				tl.sleepUntil(anchor + seek.Microseconds())
				anchor -= 10_000_000
				end -= 10_000_000
				tl.clear(true, "seek 10 s ahead")
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

// publish sends the beats of tr still ahead, with the track's media time zero at anchor.
func (tl *timeline) publish(tr track, anchor int64, pickup int) {
	period := time.Duration(float64(time.Minute) / tr.bpm)
	now := tl.now()
	for k := 0; ; k++ {
		pos := firstAt + time.Duration(k)*period
		if pos >= tr.length {
			return
		}
		ts := anchor + pos.Microseconds()
		if ts <= now {
			continue
		}
		down := k >= pickup && (k-pickup)%4 == 0
		frame := binary.BigEndian.AppendUint64([]byte{beatTag}, uint64(ts))
		if down {
			frame = append(frame, 1)
		} else {
			frame = append(frame, 0)
		}
		if tl.log {
			mark := ""
			if down {
				mark = " down"
			}
			fmt.Printf("beat %s %g%s\n", tl.wall(ts).Format("15:04:05.000"), tr.bpm, mark)
		}
		tl.mu.Lock()
		tl.beats = append(tl.beats, frame)
		tl.mu.Unlock()
		tl.fan(frame, false)
	}
}

// clear starts a new timeline, like MA re-anchoring: the old beats no longer apply.
func (tl *timeline) clear(playing bool, why string) {
	fmt.Printf("%s  %s\n", time.Now().Format("15:04:05.000"), why)
	tl.mu.Lock()
	tl.playing, tl.beats = playing, nil
	tl.mu.Unlock()
	tl.fan([]byte(`{"type": "stream/clear"}`), true)
}

func (tl *timeline) end() {
	fmt.Printf("%s  stopped\n", time.Now().Format("15:04:05.000"))
	tl.mu.Lock()
	tl.playing, tl.beats = false, nil
	tl.mu.Unlock()
	tl.fan([]byte(`{"type": "stream/end"}`), true)
}

// waveform sends silent waveform frames about 43 times a second while a track plays.
func (tl *timeline) waveform() {
	silent := make([]byte, 1024)
	for i := range silent {
		silent[i] = 0x80
	}
	for range time.Tick(time.Second / 43) {
		tl.mu.Lock()
		playing := tl.playing
		tl.mu.Unlock()
		if playing {
			ts := tl.now() + waveLead.Microseconds()
			tl.fan(append(binary.BigEndian.AppendUint64([]byte{waveTag}, uint64(ts)), silent...), false)
		}
	}
}

// join adds a viewer and gives it the beats still ahead, under one lock so none is missed.
func (tl *timeline) join(v *viewer) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	now := tl.now()
	for _, f := range tl.beats {
		if int64(binary.BigEndian.Uint64(f[1:9])) > now {
			v.out <- outMsg{false, f}
		}
	}
	tl.viewers[v] = true
}
