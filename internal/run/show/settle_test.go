package show

import "testing"

func TestSettle(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	if ps := p.Settle(at(10_000)); ps != nil {
		t.Errorf("no beat yet: %v", ps)
	}
	p.Beat(moment(4), 0) // a moment at 2 s: lamp-4 starts up
	p.Beat(moment(5), 0) // the latest beat, at 2.5 s
	if ps := p.Settle(at(2900)); ps != nil {
		t.Errorf("within a beat of the latest one: %v", ps)
	}
	ps := p.Settle(at(3100))
	if len(ps) != 1 || ps[0].Light != "lamp-4" || ps[0].Payload["brightness"] != level(10) || !ps[0].Land.Equal(at(3100)) {
		t.Fatalf("a gap settles the breathe light at its low end: %+v", ps)
	}
	if again := p.Settle(at(9000)); again != nil {
		t.Errorf("once per gap: %v", again)
	}
	for _, m := range p.Beat(moment(8), 0) {
		if m.Light == "lamp-4" && m.Land.Equal(moment(8).At) && m.Payload["brightness"] != level(100) {
			t.Errorf("the moment after a gap pulses up from low: %v", m.Payload)
		}
	}
	if ps := p.Settle(at(4400)); ps != nil {
		t.Errorf("within a beat of the new beat: %v", ps)
	}
	if ps := p.Settle(at(4600)); len(ps) != 1 {
		t.Errorf("the next gap settles again: %v", ps)
	}
}

func TestSettleWithoutBreatheLights(t *testing.T) {
	cfg := testConfig("separate")
	cfg.Fixtures = cfg.Fixtures[:1]
	p := NewPlanner(cfg)
	p.Beat(moment(4), 0)
	if ps := p.Settle(at(9000)); ps != nil {
		t.Errorf("only colour lights: %v", ps)
	}
}
