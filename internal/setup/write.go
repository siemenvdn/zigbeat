package setup

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/siemenvdn/zigbeat/internal/config"
)

// renderFixtures is the fixtures block of the configuration.
func renderFixtures(fs []config.Fixture) string {
	var b strings.Builder
	b.WriteString("fixtures:\n")
	for _, f := range fs {
		fmt.Fprintf(&b, "  - name: %s\n    role: %s\n", yamlName(f.Name), f.Role)
		if f.Mode != "" && f.Mode != config.Alternate { // the default is left out {
			fmt.Fprintf(&b, "    mode: %s\n", f.Mode)
		}
		names := make([]string, len(f.Lights))
		for i, l := range f.Lights {
			names[i] = yamlName(l)
		}
		fmt.Fprintf(&b, "    lights: [%s]\n", strings.Join(names, ", "))
	}
	return b.String()
}

// flowUnsafe are characters a plain name cannot hold in a flow sequence.
const flowUnsafe = ",[]{}#&*!|>'\"%@`"

// yamlName writes a Zigbee2MQTT name plain when YAML reads it back unchanged,
// as a value and in a flow sequence, else in double quotes; JSON strings are valid YAML.
func yamlName(s string) string {
	var flow []string
	if out, err := yaml.Marshal(s); err == nil && string(out) == s+"\n" && !strings.ContainsAny(s, flowUnsafe) &&
		yaml.Unmarshal([]byte("["+s+"]"), &flow) == nil && len(flow) == 1 && flow[0] == s {
		return s
	}
	q, _ := json.Marshal(s)
	return string(q)
}

// canaryLine is the canary of a config setup writes.
func canaryLine(canary string) string {
	return "canary: " + yamlName(canary) + "  # read only, never changed"
}

// newConfig is the text of a new configuration at path: the MQTT settings, a
// budget for zigbeat probe to fill in, the canary, the fixtures, and the
// optional blocks as comments.
func newConfig(path string, m config.MQTT, canary string, fs []config.Fixture, today time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# zigbeat configuration, written by zigbeat setup on %s.\n", today.Format(time.DateOnly))
	b.WriteString("# Lights not listed under fixtures are never touched.\n\nmqtt:\n")
	fmt.Fprintf(&b, "  broker: %s\n", yamlName(m.Broker))
	if m.Username != "" {
		fmt.Fprintf(&b, "  username: %s\n  password_env: %s\n", yamlName(m.Username), yamlName(m.PasswordEnv))
	}
	fmt.Fprintf(&b, "  base_topic: %s\n\n", yamlName(m.BaseTopic))
	b.WriteString("budget:\n")
	fmt.Fprintf(&b, "  max_rate: 0          # frames per second: zigbeat probe -config %s fills it in\n", path)
	b.WriteString("  latency_ms: 0        # p50 per frame at that load: zigbeat probe fills it in\n")
	b.WriteString("  audio_offset_ms: 0   # positive when the speakers play later; type save while calibrating in zigbeat run\n")
	if canary != "" {
		b.WriteString("\n" + canaryLine(canary) + "\n")
	}
	b.WriteString("\n" + renderFixtures(fs))
	e := config.DefaultEffects
	fmt.Fprintf(&b, "\n# effects:\n#   pace: %d\n#   brightness: %d\n#   breathe_range: [%d, %d]\n#   breathe_variant: %s\n",
		e.Pace, e.Brightness, e.BreatheRange[0], e.BreatheRange[1], e.BreatheVariant)
	fmt.Fprintf(&b, "#   album_palette: %t\n#   album_colors: %d\n#   variant: %s\n", e.AlbumPalette, e.AlbumColors, e.Variant)
	b.WriteString("\n# music_assistant:\n#   url: ws://music-assistant:8095\n#   token_env: ZIGBEAT_MA_TOKEN\n")
	return b.String()
}
