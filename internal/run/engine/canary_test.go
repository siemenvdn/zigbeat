package engine

import (
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

func TestCanary(t *testing.T) {
	cfg := testConfig("separate")
	cfg.Canary = "far-lamp"
	cfg.Fixtures[1].Lights = []string{"lamp-4", "lamp-5", "lamp-6", "lamp-7"}
	e := New(cfg, 200*time.Millisecond, t0)
	got := e.Due(t0)
	if len(got) == 0 || got[0].Light != "far-lamp" || !got[0].Get {
		t.Fatalf("first read goes to the canary: %+v", got)
	}
	if len(got) != flow.MaxInFlight {
		t.Fatalf("the canary read takes one of the %d slots: %q", flow.MaxInFlight, lightsOf(got))
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
