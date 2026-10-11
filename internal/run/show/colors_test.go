package show

import "testing"

func TestNoRepeatedColour(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Start(t0)
	for i := 0; i < 30; i++ {
		before := map[string]string{}
		for l, c := range p.current {
			before[l] = c
		}
		for _, pl := range p.Beat(moment(4*i), 0) {
			if c, ok := pl.Payload["color"].(map[string]string); ok && c["hex"] == before[pl.Light] {
				t.Fatalf("moment %d: %s got its own colour %s again", i, pl.Light, c["hex"])
			}
		}
		if p.current["lamp-1"] == p.current["lamp-2"] || p.current["lamp-2"] == p.current["lamp-3"] || p.current["lamp-1"] == p.current["lamp-3"] {
			t.Fatalf("moment %d: neighbours share a colour: %v", i, p.current)
		}
	}
}
