package run

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 11, 21, 0, 0, 0, time.UTC)

func at(ms int) time.Time { return t0.Add(time.Duration(ms) * time.Millisecond) }

func TestLimiter(t *testing.T) {
	l := newLimiter(10, t0)
	if !l.take(t0, 6) {
		t.Fatal("a full bucket holds one moment of 6 frames")
	}
	if l.take(t0, 1) {
		t.Fatal("bucket should be empty")
	}
	if l.take(at(50), 1) {
		t.Fatal("50 ms at 10/s refills only half a frame")
	}
	if !l.take(at(100), 1) {
		t.Fatal("100 ms at 10/s refills one frame")
	}
	if !l.take(at(10_000), 6) || l.take(at(10_000), 1) {
		t.Fatal("refill must stop at the burst size")
	}
}
