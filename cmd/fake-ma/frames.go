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

// waveform sends silent waveform frames waveRate times a second while a track plays.
func (tl *timeline) waveform() {
	silent := make([]byte, waveSamples)
	for i := range silent {
		silent[i] = silentSample
	}
	for range time.Tick(time.Second / waveRate) {
		tl.mu.Lock()
		playing := tl.playing
		tl.mu.Unlock()
		if playing {
			ts := tl.now() + waveLead.Microseconds()
			tl.fan(waveFrame(ts, silent), false)
		}
	}
}
