package ma

import (
	"math"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
)

const (
	waveSamples = 1024
	sampleZero  = 128
	sampleScale = 127
)

const (
	lowCut      = 150
	assumedRate = 44_100
	minRate     = 32_000
	maxRate     = 192_000

	stampSlack = 1.001
	keepFrames = 2048
	silenceDB  = -45
	minSpread  = 6
	maxSpread  = 15
)

type level struct {
	ts int64
	ms float64
}

type meter struct {
	a, y1, y2 float64
	prev      int64
	frames    []level
	lo, hi    float64
}

func newMeter() *meter {
	m := &meter{}
	m.reset()
	return m
}

func (m *meter) reset() {
	*m = meter{a: coefficient(assumedRate), lo: math.Inf(1), hi: math.Inf(-1)}
}

func coefficient(rate float64) float64 { return 1 - math.Exp(-2*math.Pi*lowCut/rate) }

func (m *meter) add(ts int64, samples []byte) {
	if m.prev != 0 && ts > m.prev {
		rate := float64(len(samples)) / time.Duration((ts-m.prev)*int64(time.Microsecond)).Seconds()
		if rate >= minRate/stampSlack && rate <= maxRate*stampSlack {
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
