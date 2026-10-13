package engine

import (
	"strings"
	"testing"
	"time"
)

func TestStatus(t *testing.T) {
	e := started(t, "separate", 100)
	line := e.Stats(at(10_000)).Line()
	for _, want := range []string{"21:00:10", "moments 0", "dropped 0", "in flight 0", "p50 20 ms", "backoff 0", "canary -", "z2m err 0"} {
		if !strings.Contains(line, want) {
			t.Errorf("status %q lacks %q", line, want)
		}
	}
}

func TestStats(t *testing.T) {
	e := started(t, "separate", 100)
	e.lights["lamp-3"].away, e.lights["lamp-3"].retryAt = true, at(70_000)
	e.bo.Pause(at(10_000))
	st := e.Stats(at(10_000))
	if st.Lights != 3 || !st.Paused() || st.P50 != 20*time.Millisecond || st.FramesPerSec <= 0 {
		t.Errorf("stats %+v", st)
	}
	if again := e.Stats(at(20_000)); again.FramesPerSec != 0 {
		t.Errorf("the frame counter resets per call: %+v", again)
	}
}
