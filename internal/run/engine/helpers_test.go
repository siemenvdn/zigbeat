package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
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
	return startedWith(t, cfg)
}

// startedWith is started for any config.
func startedWith(t *testing.T, cfg config.Config) *Engine {
	t.Helper()
	e := New(cfg, 0, t0)
	queued := func() bool {
		for _, q := range e.queue {
			if len(q) > 0 {
				return true
			}
		}
		return false
	}
	for i := 0; i < 1000 && queued(); i++ {
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
