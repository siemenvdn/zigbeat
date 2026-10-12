package show

import "testing"

func TestKickAndReturn(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	ps := p.Beat(moment(0), 0)
	kick, ret := ps[0], ps[1]
	if kick.Payload["brightness"] != 229 || !kick.Drop || !kick.Land.Equal(t0) {
		t.Errorf("kick: %+v", kick)
	}
	if ret.Payload["brightness"] != 152 || ret.Drop || !ret.Land.Equal(at(500)) {
		t.Errorf("return: %+v", ret)
	}
	for _, pl := range p.Beat(moment(4), 1) {
		if _, ok := pl.Payload["color"]; ok && pl.Payload["brightness"] != nil {
			t.Errorf("backoff 1 has no kick: %v", pl.Payload)
		}
	}
}
