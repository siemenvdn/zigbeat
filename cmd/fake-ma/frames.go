package main

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/music"
)

// The relay's frames, [tag][ts:8] and a payload, as MA's tap.py writes them:
// copied on purpose, so the fake does not share zigbeat's decoder.
const (
	beatTag      = 17
	waveTag      = 22
	headerLen    = 1 + 8
	downBit      = 1    // the flags byte of a downbeat
	waveSamples  = 1024 // samples per waveform frame
	silentSample = 0x80 // the midpoint of unsigned 8-bit PCM
	sampleRate   = 44100
	waveRate     = sampleRate / waveSamples // about 43 waveform frames a second
)

// waveLead is how far ahead MA stamps waveform frames.
const waveLead = 5 * time.Second

func beatFrame(ts int64, down bool) []byte {
	var flags byte
	if down {
		flags = downBit
	}
	return append(binary.BigEndian.AppendUint64([]byte{beatTag}, uint64(ts)), flags)
}

func waveFrame(ts int64, samples []byte) []byte {
	return append(binary.BigEndian.AppendUint64([]byte{waveTag}, uint64(ts)), samples...)
}

// frameTime is the timestamp of a frame, in µs on the fake's clock.
func frameTime(f []byte) int64 { return int64(binary.BigEndian.Uint64(f[1:headerLen])) }

// music starts the waveform of tr with its media time zero at anchor, from
// where it is now: at a track's start that is 0, so the first waveLead goes at once.
func (tl *timeline) music(tr track, pickup int, anchor int64) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.cur, tl.pickup, tl.anchor = tr, pickup, anchor
	tl.next = max(0, (tl.now()-anchor)*sampleRate/time.Second.Microseconds())
}

// publish sends the beats of tr still ahead, with the track's media time zero at anchor.
func (tl *timeline) publish(tr track, anchor int64, pickup int) {
	period := tr.period()
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
		down := k >= pickup && (k-pickup)%music.PerBar == 0
		frame := beatFrame(ts, down)
		if tl.log {
			mark := ""
			if down {
				mark = " down"
			}
			fmt.Printf("beat %s %g%s\n", tl.wall(ts).Format(cli.ClockMS), tr.bpm, mark)
		}
		tl.mu.Lock()
		tl.beats = append(tl.beats, frame)
		tl.mu.Unlock()
		tl.fan(frame, false)
	}
}

// waveform sends the track's music while it plays, as MA's tap does: windows
// of waveSamples one after the other, each stamped at its end, up to waveLead ahead.
func (tl *timeline) waveform() {
	for range time.Tick(time.Second / waveRate) {
		tl.mu.Lock()
		var frames [][]byte
		for tl.playing {
			end := tl.next + waveSamples
			ts := tl.anchor + end*time.Second.Microseconds()/sampleRate
			if ts > tl.now()+waveLead.Microseconds() {
				break
			}
			frames = append(frames, waveFrame(ts, synth(tl.cur, tl.pickup, tl.next, tl.noise)))
			tl.next = end
		}
		tl.mu.Unlock()
		for _, f := range frames {
			tl.fan(f, false)
		}
	}
}
