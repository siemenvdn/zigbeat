package run

import "testing"

func TestParseOptions(t *testing.T) {
	o, err := ParseOptions("run", "", nil)
	if err != nil || o.Config != "config.yaml" || o.BPM != 120 {
		t.Fatalf("defaults: %+v, %v", o, err)
	}
	for _, bpm := range []string{"29", "301"} {
		if _, err := ParseOptions("serve", "", []string{"-bpm", bpm}); err == nil {
			t.Errorf("-bpm %s must be rejected", bpm)
		}
	}
}
