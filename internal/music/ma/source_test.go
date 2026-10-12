package ma

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
)

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
	events, notes := listen(t, f, "", "good")
	(<-f.conns).Close()
	notes.waitFor(t, "connection lost")
	c := <-f.conns
	notes.waitFor(t, "connected")
	_ = c.WriteBinary(beatFrame(f.now()+600_000, true))
	if b := next(t, events); b.Index != 0 {
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
		events := Source{URL: f.url, Token: "good", Notes: notes}.Events(ctx)
		notes.waitFor(t, "connected")
		cancel()
		for range events {
		}
		if n := f.dials.Load(); n != int64(i+1) {
			t.Fatalf("after %d stops %d dials: a stopped source must not dial again", i+1, n)
		}
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
