package flow

import (
	"testing"
)

func TestLimiter(t *testing.T) {
	l := NewLimiter(10, t0)
	if !l.Take(t0, 6) {
		t.Fatal("a full bucket holds one moment of 6 frames")
	}
	if l.Take(t0, 1) {
		t.Fatal("bucket should be empty")
	}
	if l.Take(at(50), 1) {
		t.Fatal("50 ms at 10/s refills only half a frame")
	}
	if !l.Take(at(100), 1) {
		t.Fatal("100 ms at 10/s refills one frame")
	}
	if !l.Take(at(10_000), 6) || l.Take(at(10_000), 1) {
		t.Fatal("refill must stop at the burst size")
	}
}
