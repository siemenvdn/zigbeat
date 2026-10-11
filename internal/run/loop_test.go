package run

import (
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/run/engine"
)

func TestDrainSurvivesAPanickedEngine(t *testing.T) {
	r, _ := newShell(t)
	r.eng = &engine.Engine{} // broken: any call into it panics
	r.drain(50 * time.Millisecond)
}
