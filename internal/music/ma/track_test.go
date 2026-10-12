package ma

import (
	"slices"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/ws"
)

const (
	navy    = `{"type":"color","payload":{"primary":[32,45,89],"accent":[219,128,52],"on_dark":null}}`
	red     = `{"type":"color","payload":{"primary":[180,30,40]}}`
	allNull = `{"type":"color","payload":{"primary":null,"accent":null,"on_dark":null}}`
)

var navyRGB = []music.RGB{{32, 45, 89}, {219, 128, 52}}

// nextTrack is the next track; beats are skipped, a clear fails the test.
func nextTrack(t *testing.T, events <-chan music.Event) music.Track {
	t.Helper()
	for {
		switch ev := nextEvent(t, events).(type) {
		case music.Track:
			return ev
		case music.Clear:
			t.Fatal("a clear instead of a track")
		}
	}
}

// burst sends the beats of a short track, beginning on a downbeat 600 ms ahead.
func burst(f *fakeMA, c *ws.Conn) {
	start := f.now() + 600_000
	for i := range 4 {
		_ = c.WriteBinary(beatFrame(start+int64(i)*100_000, i == 0))
	}
}

func withGrace(t *testing.T, d time.Duration) {
	old := nullGrace
	nullGrace = d
	t.Cleanup(func() { nullGrace = old })
}

func TestPaletteBeforeTheBurstIsInTheFirstTrack(t *testing.T) {
	f := startMA(t)
	events, notes := listen(t, f, "", "good")
	c := <-f.conns
	notes.waitFor(t, "connected")
	_ = c.WriteText([]byte(navy))
	burst(f, c)
	if ev := nextEvent(t, events); !slices.Equal(ev.(music.Track).Palette, navyRGB) {
		t.Fatalf("the first event is the track with its colours: %+v", ev)
	}
	if _, ok := nextEvent(t, events).(music.Beat); !ok {
		t.Error("then its beats")
	}
}

func TestNullCountsAfterTheGrace(t *testing.T) {
	withGrace(t, 300*time.Millisecond)
	f := startMA(t)
	events, notes := listen(t, f, "", "good")
	c := <-f.conns
	notes.waitFor(t, "connected")
	burst(f, c)
	if tr := nextTrack(t, events); tr.Palette != nil {
		t.Fatalf("no colours yet: %+v", tr)
	}
	_ = c.WriteText([]byte(navy))
	if tr := nextTrack(t, events); !slices.Equal(tr.Palette, navyRGB) {
		t.Fatalf("a real palette counts at once: %+v", tr)
	}
	sent := time.Now()
	_ = c.WriteText([]byte(allNull))
	if tr := nextTrack(t, events); tr.Palette != nil || time.Since(sent) < 250*time.Millisecond {
		t.Errorf("null counts only after the grace: %+v after %s", tr, time.Since(sent))
	}
}

func TestRealPaletteWithinTheGrace(t *testing.T) {
	withGrace(t, 300*time.Millisecond)
	f := startMA(t)
	events, notes := listen(t, f, "", "good")
	c := <-f.conns
	notes.waitFor(t, "connected")
	_ = c.WriteText([]byte(navy))
	burst(f, c)
	nextTrack(t, events)
	_ = c.WriteText([]byte(allNull))
	_ = c.WriteText([]byte(red))
	_ = c.WriteText([]byte(red))
	if tr := nextTrack(t, events); !slices.Equal(tr.Palette, []music.RGB{{180, 30, 40}}) {
		t.Fatalf("the next cover, without the null in between: %+v", tr)
	}
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case ev := <-events:
			if tr, ok := ev.(music.Track); ok {
				t.Fatalf("no second track for the same palette, nor a null one: %+v", tr)
			}
		case <-deadline:
			return
		}
	}
}

func TestClearKeepsThePalette(t *testing.T) {
	f := startMA(t)
	events, notes := listen(t, f, "", "good")
	c := <-f.conns
	notes.waitFor(t, "connected")
	_ = c.WriteText([]byte(navy))
	burst(f, c)
	nextTrack(t, events)
	next(t, events)
	_ = c.WriteText([]byte(`{"type": "stream/clear"}`)) // a seek: MA sends no colour
	if ev := nextEvent(t, events); ev != (music.Clear{}) {
		t.Fatalf("the clear: %+v", ev)
	}
	burst(f, c)
	if tr := nextTrack(t, events); !slices.Equal(tr.Palette, navyRGB) {
		t.Errorf("after a seek the track keeps its colours: %+v", tr)
	}
}
