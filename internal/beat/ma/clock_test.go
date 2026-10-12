package ma

import (
	"testing"
	"time"
)

func TestClockAnswer(t *testing.T) {
	c := &clock{base: time.Unix(1000, 0)}
	if _, ok := c.at(5); ok {
		t.Fatal("no estimate before the first answer")
	}
	// MA's clock is 37 s ahead; 100 µs each way, 100 µs inside MA.
	c.add(1000, 37_000_000+1100, 37_000_000+1200, 1300)
	e, ok := c.best()
	if !ok || e.offset != 37_000_000 || e.rtt != 200 {
		t.Fatalf("estimate %+v", e)
	}
	if at, _ := c.at(37_000_000 + 2_000_000); !at.Equal(time.Unix(1002, 0)) {
		t.Errorf("at %v, want 2 s after base", at)
	}
}

func TestClockShortestRoundTripWins(t *testing.T) {
	c := &clock{}
	c.burst()
	c.add(0, 1_000_500, 1_000_500, 1000) // rtt 1000, offset 1_000_000
	c.add(0, 2_000_050, 2_000_050, 100)  // rtt 100, offset 2_000_000
	c.burst()
	c.add(0, 3_000_250, 3_000_250, 500) // rtt 500
	if e, _ := c.best(); e.offset != 2_000_000 {
		t.Errorf("best %+v, want the 100 µs answer", e)
	}
	for range keepBursts {
		c.burst()
		c.add(0, 4_000_400, 4_000_400, 800) // rtt 800
	}
	if e, _ := c.best(); e.offset != 4_000_000 {
		t.Errorf("best %+v: bursts older than %d are forgotten", e, keepBursts)
	}
}
