package main

import (
	"encoding/binary"
	"time"
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
