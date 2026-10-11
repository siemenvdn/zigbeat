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
	if _, gone := e.away["lamp-3"]; !gone || e.bo.Level() != 1 {
		t.Fatalf("second loss in a row: away %v, level %d", e.away, e.bo.Level())
	}
	e.Beat(at(36_000), moment(80))
	if len(e.queue["lamp-3"]) != 0 {
		t.Fatal("nothing is planned for a light that is left alone")
	}
	got := e.Due(at(95_000))
	if len(got) == 0 || got[0].Light != "lamp-3" || !got[0].Get || strings.Count(lightsOf(got), "lamp-3") != 1 {
		t.Fatalf("a minute later it is read once: %+v", got)
	}
	e.State("lamp-3", at(95_040))
	if _, gone := e.away["lamp-3"]; gone || e.fails["lamp-3"] != 0 {
		t.Fatal("an answer brings it back")
	}
}
