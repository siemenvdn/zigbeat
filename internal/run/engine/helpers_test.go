package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/run/runtest"
)

var (
	t0         = runtest.T0
	at         = runtest.At
	testConfig = runtest.Config
	moment     = runtest.Beat
)

func lightsOf(ss []Send) string {
	var ls []string
	for _, s := range ss {
		ls = append(ls, s.Light)
	}
	return strings.Join(ls, ",")
}

// started returns an engine whose start pass went out and was confirmed after 40 ms.
func started(t *testing.T, mode string, maxRate float64) *Engine {
	t.Helper()
	cfg := testConfig(mode)
	cfg.Budget.MaxRate = maxRate
	e := New(cfg, 0, t0)
	for i := 0; i < 1000 && len(e.queue["lamp-4"])+len(e.queue["lamp-1"])+len(e.queue["lamp-2"])+len(e.queue["lamp-3"]) > 0; i++ {
		now := at(i * 50)
		for _, s := range e.Due(now) {
			e.State(s.Light, now.Add(40*time.Millisecond))
		}
	}
	if e.InFlight() != 0 || e.bo.Level() != 0 {
		t.Fatalf("start pass did not settle: in flight %d, backoff %d", e.InFlight(), e.bo.Level())
	}
	return e
}
