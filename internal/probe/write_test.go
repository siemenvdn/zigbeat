package probe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/probe/measure"
	"github.com/siemenvdn/zigbeat/internal/probe/report"
)

const draft = `budget:
  max_rate: 0          # frames per second: zigbeat probe -config config.yaml fills it in
  latency_ms: 0        # p50 per frame at that load: zigbeat probe fills it in
  audio_offset_ms: 0   # positive when the speakers play later

canary: far-lamp

fixtures:
  - name: pendant
    role: color
    lights: [lamp-1, lamp-2]
  - name: floor
    role: breathe
    lights: [lamp-4]
`

func writeConfig(t *testing.T, yaml string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWriteBudget(t *testing.T) {
	b := report.Budget{Clean: measure.Step{Rate: 30}, Rate: 15, Ref: measure.Step{Rate: 12, Cmd: measure.Stats{P50: 66 * time.Millisecond}}}
	day := time.Date(2026, 10, 12, 21, 0, 0, 0, time.UTC)
	cases := map[string]string{
		"color": "  max_rate: 15         # zigbeat probe 2026-10-12: the clean rate of 30 cmd/s in color mode / 2\n" +
			"  latency_ms: 66       # zigbeat probe 2026-10-12: p50 per frame at 12 cmd/s\n",
		"combo": "  max_rate: 30         # zigbeat probe 2026-10-12: the clean rate of 30 cmd/s in combo mode / 2\n" +
			"  latency_ms: 33       # zigbeat probe 2026-10-12: p50 per frame at 12 cmd/s\n",
	}
	before := "  max_rate: 0          # frames per second: zigbeat probe -config config.yaml fills it in\n" +
		"  latency_ms: 0        # p50 per frame at that load: zigbeat probe fills it in\n"
	for mode, after := range cases {
		path := writeConfig(t, draft)
		if _, _, err := writeBudget(path, b, mode, day); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(path)
		if want := strings.Replace(draft, before, after, 1); string(data) != want {
			t.Errorf("%s: got\n%s\nwant\n%s", mode, data, want)
		}
	}
	path := writeConfig(t, strings.Replace(draft, "max_rate: 0", `max_rate: "0"`, 1))
	if rate, latency, err := writeBudget(path, b, "color", day); err == nil || rate != 15 || latency != 66*time.Millisecond {
		t.Errorf("a quoted value must fail with the values: %g, %v, %v", rate, latency, err)
	}
}

func TestFromConfig(t *testing.T) {
	path := writeConfig(t, draft)
	o, err := parseFlags([]string{"-config", path})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(o.targets, ",") != "lamp-1,lamp-2" || o.mode != "color" || o.canary != "far-lamp" || o.broker != "127.0.0.1:1883" || o.fallback {
		t.Errorf("from the config: %+v", o)
	}
	if o, err = parseFlags([]string{"-config", path, "-targets", "lamp-2", "-canary", "lamp-1"}); err != nil || strings.Join(o.targets, ",") != "lamp-2" || o.canary != "lamp-1" {
		t.Errorf("flags win: %+v, %v", o, err)
	}
	if _, err = parseFlags([]string{"-config", path, "-broker", "b:1883"}); err == nil || !strings.Contains(err.Error(), "leave out -broker") {
		t.Errorf("MQTT flags with -config: %v", err)
	}
	white := writeConfig(t, strings.Replace(draft, "    role: color\n", "    role: breathe\n", 1))
	if o, err = parseFlags([]string{"-config", white}); err != nil || strings.Join(o.targets, ",") != "lamp-1,lamp-2,lamp-4" || o.mode != "brightness" || !o.fallback {
		t.Errorf("only breathe lights: %+v, %v", o, err)
	}
	if o, err = parseFlags([]string{"-config", white, "-mode", "combo"}); err != nil || o.mode != "combo" || o.fallback {
		t.Errorf("-mode wins: %+v, %v", o, err)
	}
}
