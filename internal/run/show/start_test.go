package show

import "testing"

func TestStart(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	ps := p.Start(t0)
	if len(ps) != 4 {
		t.Fatalf("start covers every light, got %d", len(ps))
	}
	if ps[0].Payload["state"] != "ON" || ps[0].Payload["brightness"] != 152 || ps[0].Drop {
		t.Errorf("colour start: %+v", ps[0])
	}
	if ps[3].Payload["brightness"] != level(10) || ps[3].Payload["color"] != nil {
		t.Errorf("breathe start: %+v", ps[3])
	}
}
