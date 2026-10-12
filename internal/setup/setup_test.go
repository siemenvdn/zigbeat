package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

func TestSetupWritesNewConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	br := &fakeBridge{devs: home()}
	code, out := start(t, br, path, "", "1-4\n3\n")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	for _, want := range []string{
		"zigbeat setup: 8 devices, 6 lights offered",
		"  4  floor-lamp  breathe  floor-lamp  Acme white",
		"  plug-1  switch, never driven",
		"not listed: devices without a light, such as sensors and remotes (1)",
		"  3  far-lamp   Acme color",
		"far-lamp answered 3 of 3, slowest 40 ms",
		"wrote " + path + " (lights: 4, fixtures: 2, canary: far-lamp); next: zigbeat probe -config " + path,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	data, _ := os.ReadFile(path)
	m := config.MQTT{Broker: "127.0.0.1:1883", Username: "zigbeat", PasswordEnv: "ZIGBEAT_MQTT_PASSWORD", BaseTopic: "zigbee2mqtt"}
	fs := []config.Fixture{{Name: "pendant", Role: "color", Lights: []string{"pendant-1", "pendant-2", "pendant-3"}}, {Name: "floor-lamp", Role: "breathe", Lights: []string{"floor-lamp"}}}
	if want := newConfig(path, m, "far-lamp", fs, today); string(data) != want {
		t.Errorf("wrote\n%s\nwant\n%s", data, want)
	}
	if strings.Join(br.reads, ",") != "far-lamp,far-lamp,far-lamp" {
		t.Errorf("reads %v", br.reads)
	}
}

func TestFlagsAskNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	code, out := start(t, &fakeBridge{devs: home()}, path, "", "", "-lights", "pendant-1,floor-lamp", "-canary", "none")
	if code != 0 || !strings.Contains(out, "(lights: 2, fixtures: 2, canary: none)") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	code, out = start(t, &fakeBridge{devs: home()}, filepath.Join(t.TempDir(), "c.yaml"), "", "", "-lights", "pendant-1,plug-1", "-canary", "none")
	if code != 2 || !strings.Contains(out, "-lights: plug-1 is not offered (switch, never driven)") {
		t.Errorf("exit %d\n%s", code, out)
	}
	code, out = start(t, &fakeBridge{devs: home(), silent: map[string]bool{"far-lamp": true}}, filepath.Join(t.TempDir(), "c.yaml"), "", "", "-lights", "pendant-1", "-canary", "far-lamp")
	if code != 1 || !strings.Contains(out, "far-lamp answered 0 of 3") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

const existing = `# my lights

mqtt:
  broker: 127.0.0.1:1883
  username: zigbeat
  password_env: ZIGBEAT_MQTT_PASSWORD

budget:
  max_rate: 4      # from the probe
  latency_ms: 105
  audio_offset_ms: 120

canary: far-lamp   # in the hallway

# the lights in the living room
fixtures:
  - name: pendant
    role: color
    mode: together
    lights: [pendant-1, pendant-2]
  - name: floor
    role: breathe
    lights: [floor-lamp, gone-lamp]

effects:
  brightness: 70
`

func TestExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	code, out := start(t, &fakeBridge{devs: home()}, path, existing, "1-3,5\n\n")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	for _, want := range []string{
		"  1  pendant-1   color    pendant    Acme color *",
		"gone-lamp: not a Zigbee2MQTT device any more; removed from the config",
		"Enter keeps the 3 marked *",
		"Enter keeps far-lamp",
		"(lights: 4, fixtures: 2, canary: far-lamp); next: zigbeat run -config " + path,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	data, _ := os.ReadFile(path)
	want := strings.Replace(existing, `  - name: pendant
    role: color
    mode: together
    lights: [pendant-1, pendant-2]
  - name: floor
    role: breathe
    lights: [floor-lamp, gone-lamp]
`, `  - name: pendant
    role: color
    mode: together
    lights: [pendant-1, pendant-2, pendant-3]
  - name: desk-lamp
    role: color
    lights: [desk-lamp]
`, 1)
	if string(data) != want {
		t.Errorf("wrote\n%s\nwant\n%s", data, want)
	}
}

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

func TestNothingWritten(t *testing.T) {
	cases := map[string]*fakeBridge{"x\n": {devs: home()}, "": {devs: home()}, "1\n": {devs: home()}}
	for stdin, br := range cases {
		path := filepath.Join(t.TempDir(), "config.yaml")
		code, out := start(t, br, path, "", stdin)
		if code != 130 || !strings.Contains(out, "nothing was written") {
			t.Errorf("%q: exit %d\n%s", stdin, code, out)
		}
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%q: a config was written", stdin)
		}
	}
	code, out := start(t, &fakeBridge{}, filepath.Join(t.TempDir(), "config.yaml"), "", "")
	if code != 1 || !strings.Contains(out, "no retained zigbee2mqtt/bridge/devices within 5s") {
		t.Errorf("no bridge/devices: exit %d\n%s", code, out)
	}
}

func TestMQTTFlagsWithExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseFlags([]string{"-config", path, "-broker", "b:1883"}); err == nil || !strings.Contains(err.Error(), "-broker are for a new config") {
		t.Errorf("got %v", err)
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
