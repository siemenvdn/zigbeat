package ma

import (
	"encoding/binary"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/ws"
)

func waveFrame(ts int64, samples []byte) []byte {
	return append(binary.BigEndian.AppendUint64([]byte{waveTag}, uint64(ts)), samples...)
}

func TestBeatsCarryTheEnergyOfTheirBar(t *testing.T) {
	f := startMA(t)
	events, notes := listen(t, f, "", "good")
	c := <-f.conns
	notes.waitFor(t, "connected")
	const rate = 44100
	frame := int64(waveSamples * second / rate)
	start := f.now()
	loudFrom := start + 1_400_000
	n := 0
	for ts := start; ts < start+2_800_000; ts += frame {
		a := 0.05
		if ts >= loudFrom {
			a = 0.5
		}
		_ = c.WriteBinary(waveFrame(ts+frame, sine(55, a, rate, n)))
		n += waveSamples
	}
	for i := range 16 {
		_ = c.WriteBinary(beatFrame(start+600_000+int64(i)*100_000, i%music.PerBar == 0))
	}
	first, last := next(t, events), music.Beat{}
	for range 15 {
		last = next(t, events)
	}
	if first.Energy != music.UnknownEnergy || last.Energy < 0.95 {
		t.Errorf("the quiet start is unknown, the loud end near 1: %.3f, then %.3f", first.Energy, last.Energy)
	}
}

func TestEmptyWaveformIsSkipped(t *testing.T) {
	if m := parse(ws.Message{Binary: true, Data: waveFrame(1, nil)}, 0); m.kind != kindOther {
		t.Errorf("a waveform frame without samples would make energy NaN: kind %d", m.kind)
	}
}
