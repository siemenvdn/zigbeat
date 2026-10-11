package run

import "testing"

func TestParseFlags(t *testing.T) {
	o, err := parseFlags(nil)
	if err != nil || o.config != "config.yaml" || o.bpm != 120 {
		t.Fatalf("defaults: %+v, %v", o, err)
	}
	for _, bpm := range []string{"29", "301"} {
		if _, err := parseFlags([]string{"-bpm", bpm}); err == nil {
			t.Errorf("-bpm %s must be rejected", bpm)
		}
	}
}
