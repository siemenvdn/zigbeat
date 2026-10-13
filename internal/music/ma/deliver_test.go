package ma

import (
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
)

func TestBeatsAheadAndOnTime(t *testing.T) {
	f := startMA(t)
	events, notes := listen(t, f, "", "good")
	c := <-f.conns
	notes.waitFor(t, "connected, clock round trip")
	start := f.now() + 600_000
	var want []int64
	for i := range 4 {
		ts := start + int64(i)*100_000
		want = append(want, ts)
		_ = c.WriteBinary(beatFrame(ts, i == 1))
	}
	if tr, ok := nextEvent(t, events).(music.Track); !ok || tr.BPM != 600 {
		t.Fatalf("the track at 600 BPM comes before its first beat: %+v", tr)
	}
	for i, ts := range want {
		b := next(t, events)
		if ahead := time.Until(b.At); ahead < lead*8/10 {
			t.Errorf("beat %d only %s ahead", i, ahead)
		}
		if d := b.At.Sub(f.local(ts)); d < -5*time.Millisecond || d > 5*time.Millisecond {
			t.Errorf("beat %d at %s off its time on the fake's clock", i, d)
		}
		if b.Index != i-1 {
			t.Errorf("beat %d: %+v", i, b)
		}
	}
	notes.waitFor(t, "4 beats ahead, 600.0 BPM, 1 downbeats")
}

func TestLateBeatIsSkipped(t *testing.T) {
	f := startMA(t)
	events, notes := listen(t, f, "", "good")
	c := <-f.conns
	notes.waitFor(t, "connected")
	now := f.now()
	_ = c.WriteBinary(beatFrame(now+100_000, true))
	_ = c.WriteBinary(beatFrame(now+700_000, false))
	if b := next(t, events); b.Index != 1 {
		t.Errorf("the late downbeat is skipped but counted: %+v", b)
	}
}

func TestClearAndEndDropTheBeats(t *testing.T) {
	f := startMA(t)
	events, notes := listen(t, f, "", "good")
	c := <-f.conns
	notes.waitFor(t, "connected")
	_ = c.WriteBinary(beatFrame(f.now()+800_000, true))
	_ = c.WriteText([]byte(`{"type": "stream/clear"}`))
	notes.waitFor(t, "new track or seek, waiting for beats")
	_ = c.WriteBinary(beatFrame(f.now()+700_000, false))
	_ = c.WriteBinary(beatFrame(f.now()+800_000, true))
	if b := next(t, events); b.Index != -1 || time.Until(b.At) > lead {
		t.Errorf("after a clear only the new beats count: %+v", b)
	}
	_ = c.WriteText([]byte(`{"type": "stream/end"}`))
	notes.waitFor(t, "music stopped")
	if ev := nextEvent(t, events); ev != (music.Clear{}) {
		t.Errorf("the end clears the beat handed over: %+v", ev)
	}
	select {
	case ev := <-events:
		t.Errorf("an event after the end: %+v", ev)
	case <-time.After(400 * time.Millisecond):
	}
}

func TestClearOnlyAfterABeat(t *testing.T) {
	f := startMA(t)
	events, notes := listen(t, f, "kitchen", "good")
	c := <-f.conns
	notes.waitFor(t, "connected")
	_ = c.WriteText([]byte(`{"type": "stream/clear"}`))
	notes.waitFor(t, "new track or seek")
	_ = c.WriteBinary(beatFrame(f.now()+600_000, true))
	if ev := nextEvent(t, events); ev == (music.Clear{}) {
		t.Errorf("nothing was handed over before the first clear: %+v", ev)
	}
}

func TestNumbersTheBurstAsAWhole(t *testing.T) {
	f := startMA(t)
	events, notes := listen(t, f, "", "good")
	c := <-f.conns
	notes.waitFor(t, "connected")
	now := f.now()
	_ = c.WriteBinary(beatFrame(now+lead.Microseconds()*95/100, false))
	time.Sleep(30 * time.Millisecond)
	_ = c.WriteBinary(beatFrame(now+600_000, true))
	_ = c.WriteBinary(beatFrame(now+700_000, false))
	first, second := next(t, events), next(t, events)
	if d := first.At.Sub(f.local(now + 600_000)).Abs(); first.Index != 0 || d > 5*time.Millisecond {
		t.Errorf("the downbeat comes first, as 0; the late pickup is skipped: %+v", first)
	}
	if second.Index != 1 {
		t.Errorf("the beat after the downbeat is 1, not 5: %+v", second)
	}
}
