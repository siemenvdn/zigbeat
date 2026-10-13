package flow

import (
	"testing"
	"time"
)

func TestLatencyPerFrameAndRising(t *testing.T) {
	ms := time.Millisecond
	f := NewInFlight(50 * ms)
	if f.Estimate() != 50*ms {
		t.Fatalf("before minSamples the budget's latency counts: %v", f.Estimate())
	}

	for i := range samples {
		now := at(i * 1000)
		f.Add("lamp-1", now, 1)
		f.Add("lamp-2", now, 2)
		f.Confirm("lamp-1", now.Add(40*ms))
		f.Confirm("lamp-2", now.Add(120*ms))
	}
	if d, n := f.Median(); n != samples || d != 60*ms {
		t.Errorf("median per frame of the message: %v of %d, want 60 ms", d, n)
	}
	if f.Base() != 40*ms || f.Rising() {
		t.Errorf("per frame of the queue joined both took 40 ms: base %v, rising %v", f.Base(), f.Rising())
	}
	for i := range samples {
		now := at(100_000 + i*1000)
		f.Add("lamp-1", now, 1)
		f.Confirm("lamp-1", now.Add(90*ms))
	}
	if !f.Rising() {
		t.Errorf("90 ms per queued frame is more than twice the base of 40 ms: served %v", f.Served())
	}
	f.Forget()
	if f.Rising() || f.Base() != 40*ms {
		t.Error("a pause forgets the samples but keeps the base")
	}
}

func TestExpireAfterLostAfter(t *testing.T) {
	f := NewInFlight(0)
	f.Add("lamp-2", at(0), 1)
	f.Add("lamp-1", at(0), 1)
	if lost := f.Expire(at(4999)); lost != nil {
		t.Fatalf("not lost yet: %v", lost)
	}
	if lost := f.Expire(at(5000)); len(lost) != 2 || lost[0] != "lamp-1" || f.Count() != 0 {
		t.Errorf("lost %v, want both, sorted", lost)
	}
}
