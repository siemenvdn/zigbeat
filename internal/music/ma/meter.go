package ma

import (
	"math"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
)

// The relay's waveform: frames of waveSamples unsigned 8-bit samples, sampleZero ± sampleScale.
const (
	waveSamples = 1024
	sampleZero  = 128
	sampleScale = 127
)

const (
	lowCut      = 150     // Hz: kick and bass below, pads, vocals and risers above
	assumedRate = 44_100  // Hz, the rate of most music, until two frames give the real one
	minRate     = 32_000  // Hz: the lowest common music rate
	maxRate     = 192_000 // Hz: the highest Music Assistant buffers
	keepFrames  = 2048    // more than 10 s at 192 kHz; MA sends 5 s ahead
	silenceDB   = -45     // dBFS: the 8-bit noise floor is about -53
	minSpread   = 6       // dB: bars within one section differ by about 1 dB, sections by 8 to 12
	maxSpread   = 15      // dB: a fade-in or near-silence would otherwise push all music to 1
)

// level is the low band's mean square in one frame, which ends at ts.
type level struct {
	ts int64 // µs on MA's clock
	ms float64
}

// meter turns waveform frames into the energy of the coming bar, per timeline.
type meter struct {
	a, y1, y2 float64 // two one-pole low-passes at lowCut
	prev      int64   // timestamp of the previous frame; 0 before the first
	frames    []level
	lo, hi    float64 // the quietest and loudest bar so far, in dB
}

func newMeter() *meter {
	m := &meter{}
	m.reset()
	return m
}

// reset starts a new timeline.
func (m *meter) reset() {
	*m = meter{a: coefficient(assumedRate), lo: math.Inf(1), hi: math.Inf(-1)}
}

func coefficient(rate float64) float64 { return 1 - math.Exp(-2*math.Pi*lowCut/rate) }

// add takes one frame, stamped at its end. Its spacing from the previous frame
// gives the sample rate; a gap or a rate out of range keeps the last one.
func (m *meter) add(ts int64, samples []byte) {
	if m.prev != 0 && ts > m.prev {
		rate := float64(len(samples)) / time.Duration((ts-m.prev)*int64(time.Microsecond)).Seconds()
		if rate >= minRate && rate <= maxRate {
			m.a = coefficient(rate)
		}
	}
	m.prev = ts
	var sum float64
	for _, s := range samples {
		x := (float64(s) - sampleZero) / sampleScale
		m.y1 += m.a * (x - m.y1)
		m.y2 += m.a * (m.y1 - m.y2)
		sum += m.y2 * m.y2
	}
	if len(m.frames) == keepFrames {
		m.frames = m.frames[1:]
	}
	m.frames = append(m.frames, level{ts, sum / float64(len(samples))})
}

// energy is the loudness of the frames that end within (from, from+length]
// against the rest of the timeline, 0..1. Frames up to from are dropped: no
// later beat needs them.
func (m *meter) energy(from, length int64) float64 {
	i := 0
	for i < len(m.frames) && m.frames[i].ts <= from {
		i++
	}
	m.frames = m.frames[i:]
	var sum float64
	n := 0
	for _, f := range m.frames {
		if f.ts > from+length {
			break
		}
		sum, n = sum+f.ms, n+1
	}
	if n == 0 {
		return music.UnknownEnergy
	}
	l := 10 * math.Log10(sum/float64(n))
	if l < silenceDB {
		return 0
	}
	m.lo, m.hi = math.Min(m.lo, l), math.Max(m.hi, l)
	if m.hi-m.lo < minSpread {
		return music.UnknownEnergy
	}
	lo := math.Max(m.lo, m.hi-maxSpread)
	return math.Max(0, math.Min(1, (l-lo)/(m.hi-lo)))
}
