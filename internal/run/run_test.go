package run

import (
	"strings"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

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

func TestKeepLights(t *testing.T) {
	c := keepLights(testConfig("separate"), map[string]z2m.State{"lamp-2": {}, "lamp-3": {}})
	if len(c.Fixtures) != 1 || strings.Join(c.Lights(), ",") != "lamp-2,lamp-3" {
		t.Errorf("kept %v", c.Fixtures)
	}
}
