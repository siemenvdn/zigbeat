package engine

import (
	"strings"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/run/show"
)

func TestUnreachableLightIsLeftAlone(t *testing.T) {
	e := started(t, "separate", 100)
	lose := func(ms int) {
		e.enqueue([]show.Message{{Light: "lamp-3", Payload: map[string]any{"brightness": 10}, Land: at(ms)}})
		if got := lightsOf(e.Due(at(ms))); got != "lamp-3" {
			t.Fatalf("at %d ms sent %q, want lamp-3", ms, got)
		}
		e.Due(at(ms + 5_000))
	}
	lose(10_000)
	if e.bo.Level() != 1 {
		t.Fatalf("the first loss may be the network: level %d", e.bo.Level())
	}
	lose(30_000)
	if !e.Away("lamp-3") || e.bo.Level() != 1 {
		t.Fatalf("second loss in a row: away %v, level %d", e.Away("lamp-3"), e.bo.Level())
	}
	e.Music(at(36_000), moment(80))
	if len(e.lights["lamp-3"].queue) != 0 {
		t.Fatal("nothing is planned for a light that is left alone")
	}
	got := e.Due(at(95_000))
	if len(got) == 0 || got[0].Light != "lamp-3" || !got[0].Get || strings.Count(lightsOf(got), "lamp-3") != 1 {
		t.Fatalf("a minute later it is read once: %+v", got)
	}
	e.State("lamp-3", at(95_040))
	if e.Away("lamp-3") || e.lights["lamp-3"].fails != 0 {
		t.Fatal("an answer brings it back")
	}
}

func TestDisconnectedRetriesLightsLeftAlone(t *testing.T) {
	e := started(t, "separate", 100)
	for _, ms := range []int{10_000, 30_000} {
		e.enqueue([]show.Message{{Light: "lamp-3", Payload: map[string]any{"brightness": 10}, Land: at(ms)}})
		e.Due(at(ms))
		e.Due(at(ms + 5_000))
	}
	e.lights["lamp-1"].fails = 1
	e.Disconnected()
	if e.lights["lamp-1"].fails != 0 {
		t.Error("losses on a dead connection still count")
	}
	if got := e.Due(at(36_000)); len(got) == 0 || got[0].Light != "lamp-3" || !got[0].Get {
		t.Fatalf("right after the reconnect lamp-3 is read: %+v", got)
	}
}
