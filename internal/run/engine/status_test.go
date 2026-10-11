package engine

import (
	"strings"
	"testing"
)

func TestStatus(t *testing.T) {
	e := started(t, "separate", 100)
	line := e.Status(at(10_000))
	for _, want := range []string{"moments 0", "dropped 0", "in flight 0", "p50 20 ms", "backoff 0", "canary -", "z2m err 0"} {
		if !strings.Contains(line, want) {
			t.Errorf("status %q lacks %q", line, want)
		}
	}
}
