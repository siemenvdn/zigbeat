package ma

import (
	"math"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/music"
)

const second = 1_000_000

func sine(hz, a, rate float64, n int) []byte {
	out := make([]byte, waveSamples)
	for i := range out {
		x := a * math.Sin(2*math.Pi*hz*float64(n+i)/rate)
		out[i] = byte(math.RoundToEven(x*sampleScale + sampleZero))
	}
	return out
}

func feed(m *meter, hz, a, rate float64, start int64, seconds float64) int64 {
	frame := int64(waveSamples * second / rate)
	n := int(float64(start) * rate / second)
	ts := start
	for end := start + int64(seconds*second); ts < end; ts += frame {
		m.add(ts+frame, sine(hz, a, rate, n))
		n += waveSamples
	}
	return ts
}

func db(m *meter) float64 {
	var sum float64
	for _, f := range m.frames[1:] {
		sum += f.ms
	}
	return 10 * math.Log10(sum/float64(len(m.frames)-1))
}

func TestTheLowBand(t *testing.T) {
	const a = 0.5
	in := 10 * math.Log10(a*a/2)
	for _, rate := range []float64{44100, 192000} {
		low, high := newMeter(), newMeter()
		feed(low, 55, a, rate, 0, 1)
		feed(high, 2000, a, rate, 0, 1)
		if loss := in - db(low); loss < 0.5 || loss > 2 {
			t.Errorf("%g Hz: 55 Hz loses %.1f dB, want about 1", rate, loss)
		}
		if loss := in - db(high); loss < 40 {
			t.Errorf("%g Hz: 2 kHz loses only %.1f dB", rate, loss)
		}
		t.Logf("%g Hz: 55 Hz loses %.1f dB, 2 kHz %.1f dB", rate, in-db(low), in-db(high))
	}
}

func TestEnergyWithinTheTrack(t *testing.T) {
	m := newMeter()
	loud := feed(m, 55, 0.05, 44100, 0, 2)
	lower := feed(m, 55, 0.5, 44100, loud, 2)
	silent := feed(m, 55, 0.158, 44100, lower, 2)
	end := feed(m, 55, 0, 44100, silent, 2)
	if e := m.energy(0, second); e != music.UnknownEnergy {
		t.Errorf("no contrast yet: %.2f", e)
	}
	if e := m.energy(loud, second); e != 1 {
		t.Errorf("the loud part: %.2f", e)
	}
	if e := m.energy(lower, second); math.Abs(e-1.0/3) > 0.05 {
		t.Errorf("10 dB below, 20 dB of range capped at 15: %.2f, want 0.33", e)
	}
	if e := m.energy(silent, second); e != 0 {
		t.Errorf("silence: %.2f", e)
	}
	if e := m.energy(end, second); e != music.UnknownEnergy {
		t.Errorf("no frames: %.2f", e)
	}
	m.reset()
	feed(m, 55, 0.5, 44100, 0, 2)
	if e := m.energy(0, second); e != music.UnknownEnergy {
		t.Errorf("a new timeline starts over: %.2f", e)
	}
}

func TestEnergyAtAnyRate(t *testing.T) {
	var got []float64
	for _, rate := range []float64{44100, 96000} {
		m := newMeter()
		loud := feed(m, 55, 0.05, rate, 0, 2)
		middle := feed(m, 110, 0.3, rate, loud, 2)
		feed(m, 80, 0.12, rate, middle, 2)
		m.energy(0, second)
		m.energy(loud, second)
		got = append(got, m.energy(middle, second))
	}
	if math.Abs(got[0]-got[1]) > 0.03 {
		t.Errorf("44.1 kHz %.2f, 96 kHz %.2f", got[0], got[1])
	}
}

func TestMeterKeepsItsCoefficientAndBounds(t *testing.T) {
	m := newMeter()
	end := feed(m, 55, 0.5, 96000, 0, 0.1)
	a := m.a
	m.add(end+5*second, sine(55, 0.5, 96000, 0))
	if m.a != a {
		t.Errorf("a gap changed the coefficient: %g, was %g", m.a, a)
	}
	feed(m, 55, 0.5, 44100, end+6*second, 60)
	if len(m.frames) != keepFrames {
		t.Errorf("%d frames kept", len(m.frames))
	}
}
