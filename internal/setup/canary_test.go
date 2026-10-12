package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

func TestCanaryAgainAndNone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	code, out := start(t, &fakeBridge{devs: home(), silent: map[string]bool{"far-lamp": true}}, path, existing, "\n\n0\n")
	if code != 0 || !strings.Contains(out, "far-lamp answered 0 of 3; choose another canary") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "canary:") || !strings.Contains(string(data), "  audio_offset_ms: 120\n\n# the lights") {
		t.Errorf("the canary line must go, with one blank line:\n%s", data)
	}
}

func TestCanaryWithoutDefinition(t *testing.T) {
	// Zigbee2MQTT 1.x lists an unsupported router with "definition": null.
	devs := append(home(), z2m.Device{Name: "repeater", Type: "Router", Interviewed: true})
	code, out := start(t, &fakeBridge{devs: devs}, filepath.Join(t.TempDir(), "config.yaml"), "", "1\n7\n")
	if code != 0 || !strings.Contains(out, "  7  repeater") || !strings.Contains(out, "canary: repeater)") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func TestCanaryReplacedWhenQuotedOrEmpty(t *testing.T) {
	// setup itself writes an unrenamed IEEE name in quotes.
	devs := append(home(), dev("0x0017880104e45518", "plug"))
	for _, line := range []string{`canary: "0x0017880104e45518"   # in the hallway`, `canary:   # in the hallway`} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		cfg := strings.Replace(existing, "canary: far-lamp   # in the hallway", line, 1)
		code, out := start(t, &fakeBridge{devs: devs}, path, cfg, "", "-lights", "pendant-1", "-canary", "far-lamp")
		data, _ := os.ReadFile(path)
		if code != 0 || !strings.Contains(string(data), "\ncanary: far-lamp") {
			t.Errorf("%s: exit %d\n%s\n%s", line, code, out, data)
		}
	}
}
