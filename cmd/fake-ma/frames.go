package main

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/music"
)

const (
	beatTag      = 17
	waveTag      = 22
	headerLen    = 1 + 8
	downBit      = 1
	waveSamples  = 1024
	silentSample = 0x80
	sampleRate   = 44100
	waveRate     = sampleRate / waveSamples
)

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

func frameTime(f []byte) int64 { return int64(binary.BigEndian.Uint64(f[1:headerLen])) }

func (tl *timeline) music(tr track, pickup int, anchor int64) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.cur, tl.pickup, tl.anchor = tr, pickup, anchor
	tl.next = max(0, (tl.now()-anchor)*sampleRate/time.Second.Microseconds())
}

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
