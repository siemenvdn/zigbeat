package ma

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/cli"
)

const lead = 300 * time.Millisecond

// listen starts a source; the test's cleanup waits until it stopped.
func listen(t *testing.T, f *fakeMA, player, token string) (<-chan beat.Beat, *syncBuf) {
	ctx, cancel := context.WithCancel(context.Background())
	notes := &syncBuf{}
	src := Source{URL: f.url, Token: token, Player: player, Lookahead: lead, Notes: notes}
	beats := src.Beats(ctx)
	t.Cleanup(func() {
		cancel()
		for range beats {
		}
	})
	return beats, notes
}

func next(t *testing.T, beats <-chan beat.Beat) beat.Beat {
	t.Helper()
	select {
	case b := <-beats:
		return b
	case <-time.After(2 * time.Second):
		t.Fatal("no beat")
	}
	return beat.Beat{}
}

func TestBeatsAheadAndOnTime(t *testing.T) {
	f := startMA(t)
	beats, notes := listen(t, f, "", "good")
	c := <-f.conns
	notes.waitFor(t, "connected, clock round trip")
	start := f.now() + 600_000
	var want []int64
	for i := range 4 {
		ts := start + int64(i)*100_000
		want = append(want, ts)
		_ = c.WriteBinary(beatFrame(ts, i == 1)) // one beat of pickup
	}
	for i, ts := range want {
		b := next(t, beats)
		if ahead := time.Until(b.At); ahead < lead*8/10 {
			t.Errorf("beat %d only %s ahead", i, ahead)
		}
		if d := b.At.Sub(f.local(ts)); d < -5*time.Millisecond || d > 5*time.Millisecond {
			t.Errorf("beat %d at %s off its time on the fake's clock", i, d)
		}
		if b.Index != i-1 || b.BPM != 600 {
			t.Errorf("beat %d: %+v", i, b)
		}
	}
	notes.waitFor(t, "4 beats ahead, 600.0 BPM, 1 downbeats")
}

func TestLateBeatIsSkipped(t *testing.T) {
	f := startMA(t)
	beats, notes := listen(t, f, "", "good")
	c := <-f.conns
	notes.waitFor(t, "connected")
	now := f.now()
	_ = c.WriteBinary(beatFrame(now+100_000, true)) // known only 100 ms ahead
	_ = c.WriteBinary(beatFrame(now+700_000, false))
	if b := next(t, beats); b.Index != 1 {
		t.Errorf("the late downbeat is skipped but counted: %+v", b)
	}
}

func TestClearAndEndDropTheBeats(t *testing.T) {
	f := startMA(t)
	beats, notes := listen(t, f, "", "good")
	c := <-f.conns
	notes.waitFor(t, "connected")
	_ = c.WriteBinary(beatFrame(f.now()+800_000, true))
	_ = c.WriteText([]byte(`{"type": "stream/clear"}`))
	notes.waitFor(t, "new track or seek, waiting for beats")
	_ = c.WriteBinary(beatFrame(f.now()+700_000, false))
	_ = c.WriteBinary(beatFrame(f.now()+800_000, true))
	if b := next(t, beats); b.Index != -1 || time.Until(b.At) > lead {
		t.Errorf("after a clear only the new beats count: %+v", b)
	}
	_ = c.WriteText([]byte(`{"type": "stream/end"}`))
	notes.waitFor(t, "music stopped")
	if b := next(t, beats); !b.Clear {
		t.Errorf("the end clears the beat handed over: %+v", b)
	}
	select {
	case b := <-beats:
		t.Errorf("a beat after the end: %+v", b)
	case <-time.After(400 * time.Millisecond):
	}
}

func TestClearOnlyAfterABeat(t *testing.T) {
	f := startMA(t)
	beats, notes := listen(t, f, "kitchen", "good")
	c := <-f.conns
	notes.waitFor(t, "connected")
	_ = c.WriteText([]byte(`{"type": "stream/clear"}`))
	notes.waitFor(t, "new track or seek")
	_ = c.WriteBinary(beatFrame(f.now()+600_000, true))
	if b := next(t, beats); b.Clear {
		t.Errorf("nothing was handed over before the first clear: %+v", b)
	}
}

func TestRefusedAndNoPlayer(t *testing.T) {
	f := startMA(t)
	_, notes := listen(t, f, "", "bad")
	notes.waitFor(t, "token refused (Invalid or expired token)")
	_, notes = listen(t, f, "nobody", "good")
	notes.waitFor(t, "no player nobody in Music Assistant")
	_, notes = listen(t, &fakeMA{url: "ws://127.0.0.1:1"}, "", "good")
	notes.waitFor(t, "connection lost")
	plain := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(plain.Close)
	_, notes = listen(t, &fakeMA{url: "ws" + strings.TrimPrefix(plain.URL, "http")}, "", "good")
	notes.waitFor(t, "is the MilkDrop Visualizer plugin enabled?")
}

func TestReconnect(t *testing.T) {
	old := cli.RedialEvery
	cli.RedialEvery = 50 * time.Millisecond
	t.Cleanup(func() { cli.RedialEvery = old })
	f := startMA(t)
	beats, notes := listen(t, f, "", "good")
	(<-f.conns).Close()
	notes.waitFor(t, "connection lost")
	c := <-f.conns
	notes.waitFor(t, "connected")
	_ = c.WriteBinary(beatFrame(f.now()+600_000, true))
	if b := next(t, beats); b.Index != 0 {
		t.Errorf("after a reconnect: %+v", b)
	}
}

func TestNoDialAfterStop(t *testing.T) {
	f := startMA(t)
	go func() {
		for range f.conns {
		}
	}()
	const stops = 20
	for i := range stops {
		ctx, cancel := context.WithCancel(context.Background())
		notes := &syncBuf{}
		beats := Source{URL: f.url, Token: "good", Notes: notes}.Beats(ctx)
		notes.waitFor(t, "connected")
		cancel()
		for range beats {
		}
		if n := f.dials.Load(); n != int64(i+1) {
			t.Fatalf("after %d stops %d dials: a stopped source must not dial again", i+1, n)
		}
	}
}

func TestNumbersTheBurstAsAWhole(t *testing.T) {
	f := startMA(t)
	beats, notes := listen(t, f, "", "good")
	c := <-f.conns
	notes.waitFor(t, "connected")
	now := f.now()
	_ = c.WriteBinary(beatFrame(now+lead.Microseconds()*95/100, false)) // a pickup, due at once
	time.Sleep(30 * time.Millisecond)                                   // the rest of the burst comes a little later
	_ = c.WriteBinary(beatFrame(now+600_000, true))
	_ = c.WriteBinary(beatFrame(now+700_000, false))
	first, second := next(t, beats), next(t, beats)
	if d := first.At.Sub(f.local(now + 600_000)).Abs(); first.Index != 0 || d > 5*time.Millisecond {
		t.Errorf("the downbeat comes first, as 0; the late pickup is skipped: %+v", first)
	}
	if second.Index != 1 {
		t.Errorf("the beat after the downbeat is 1, not 5: %+v", second)
	}
}

func TestEndFollowsTheNextPlayer(t *testing.T) {
	old := cli.RedialEvery
	cli.RedialEvery = 50 * time.Millisecond
	t.Cleanup(func() { cli.RedialEvery = old })
	for _, c := range []struct {
		player string
		redial bool
	}{{"", true}, {"kitchen", false}} {
		f := startMA(t)
		_, notes := listen(t, f, c.player, "good")
		conn := <-f.conns
		notes.waitFor(t, "connected")
		_ = conn.WriteText([]byte(`{"type": "stream/end"}`))
		notes.waitFor(t, "music stopped")
		select {
		case <-f.conns:
			if !c.redial {
				t.Errorf("player %q: a named player keeps its connection", c.player)
			}
		case <-time.After(500 * time.Millisecond):
			if c.redial {
				t.Errorf("player %q: without a player the source dials again, so MA picks the next one that plays", c.player)
			}
		}
	}
}
