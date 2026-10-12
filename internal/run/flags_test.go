package run

import "testing"

func TestParseOptions(t *testing.T) {
	o, err := ParseOptions("run", "", nil)
	if err != nil || o.Config != "config.yaml" || o.BPM != 0 {
		t.Fatalf("defaults: %+v, %v", o, err)
	}
	if o, err := ParseOptions("run", "", []string{"-bpm", "90"}); err != nil || o.BPM != 90 {
		t.Errorf("-bpm 90: %+v, %v", o, err)
	}
	for _, bpm := range []string{"29", "301", "-5"} {
		if _, err := ParseOptions("serve", "", []string{"-bpm", bpm}); err == nil {
			t.Errorf("-bpm %s must be rejected", bpm)
		}
	}
}
