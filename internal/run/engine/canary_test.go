package engine

import (
	"testing"
	"time"
)

func TestCanary(t *testing.T) {
	cfg := testConfig("separate")
	cfg.Canary = "far-lamp"
	e := New(cfg, 200*time.Millisecond, t0)
	got := e.Due(t0)
	if len(got) == 0 || got[0].Light != "far-lamp" || !got[0].Get {
		t.Fatalf("first read goes to the canary: %+v", got)
	}
	if len(got) != 3 {
		t.Fatalf("the canary read takes one of the 3 slots: %q", lightsOf(got))
	}
	e.State("far-lamp", at(300))
	if e.bo.Level() != 1 || e.canary.Last() != 300*time.Millisecond {
		t.Fatalf("300 ms is above the 200 ms limit: level %d", e.bo.Level())
	}
	for _, s := range e.Due(at(4_999)) {
		if s.Light == "far-lamp" {
			t.Fatal("the next read waits 5 s")
		}
	}
}
