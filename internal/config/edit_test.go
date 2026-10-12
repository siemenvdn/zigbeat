package config

import (
	"strings"
	"testing"
)

func TestSetChangesOnlyTheValue(t *testing.T) {
	in := example(t)
	f := edit(t, in)
	if err := f.Set("budget.max_rate", "7.5", "zigbeat probe 2026-10-12: clean to 15 cmd/s"); err != nil {
		t.Fatal(err)
	}
	if err := f.Set("budget.latency_ms", "88", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.Set("canary", "hallway-ceiling", ""); err != nil {
		t.Fatal(err)
	}
	want := strings.NewReplacer(
		"  max_rate: 4          # Zigbee frames per second: the budget zigbeat probe suggested",
		"  max_rate: 7.5        # zigbeat probe 2026-10-12: clean to 15 cmd/s",
		"  latency_ms: 105      # p50",
		"  latency_ms: 88       # p50",
		"canary: far-lamp       # optional",
		"canary: hallway-ceiling # optional",
	).Replace(in)
	if got := string(f.Bytes()); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestSetValueWithHash(t *testing.T) {
	f := edit(t, "canary: far-lamp       # optional\n")
	if err := f.Set("canary", "lamp #2", "new"); err != nil {
		t.Fatal(err)
	}
	if got := string(f.Bytes()); got != "canary: lamp #2        # new\n" {
		t.Errorf("got %q", got)
	}
}

func TestSetFlowMapping(t *testing.T) {
	f := edit(t, "budget: {max_rate: 10, latency_ms: 50}   # flow style\n")
	if err := f.Set("budget.max_rate", "7.5", ""); err != nil {
		t.Fatal(err)
	}
	if got := string(f.Bytes()); got != "budget: {max_rate: 7.5, latency_ms: 50}  # flow style\n" {
		t.Errorf("got %q", got)
	}
	if err := f.Set("budget.audio_offset_ms", "30", ""); err == nil || !strings.Contains(err.Error(), "set budget.audio_offset_ms to 30 by hand") {
		t.Errorf("insert into a flow mapping: %v", err)
	}
}

func TestSetInserts(t *testing.T) {
	f := edit(t, "mqtt:\n  broker: b:1883\n\nbudget:\n  max_rate: 4   # frames\n  # a note\n\nfixtures: []\n")
	if err := f.Set("budget.latency_ms", "88", "p50"); err != nil {
		t.Fatal(err)
	}
	if err := f.Set("effects.brightness", "70", ""); err != nil {
		t.Fatal(err)
	}
	want := "mqtt:\n  broker: b:1883\n\nbudget:\n  max_rate: 4   # frames\n  latency_ms: 88  # p50\n  # a note\n\nfixtures: []\n\neffects:\n  brightness: 70\n"
	if got := string(f.Bytes()); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestSetRefuses(t *testing.T) {
	cases := map[string]string{
		"quoted":     "budget:\n  max_rate: \"4\"\n",
		"empty":      "budget:\n  max_rate:\n",
		"not scalar": "budget:\n  max_rate: [4]\n",
		"no mapping": "budget: 4\n",
	}
	for name, in := range cases {
		f := edit(t, in)
		err := f.Set("budget.max_rate", "15", "")
		if err == nil || !strings.HasPrefix(err.Error(), "set budget.max_rate to 15 by hand: ") {
			t.Errorf("%s: %v", name, err)
		}
		if string(f.Bytes()) != in {
			t.Errorf("%s: changed to %q", name, f.Bytes())
		}
	}
}

func TestSetBlock(t *testing.T) {
	in := example(t)
	f := edit(t, in)
	fixtures := "fixtures:\n  - name: desk\n    role: breathe\n    lights: [lamp-9]\n"
	if err := f.SetBlock("fixtures", fixtures); err != nil {
		t.Fatal(err)
	}
	if err := f.SetBlock("canary", ""); err != nil {
		t.Fatal(err)
	}
	got := string(f.Bytes())
	start := strings.Index(in, "fixtures:")
	end := strings.Index(in, "\neffects:")
	want := strings.Replace(in[:start]+fixtures+in[end:], "canary: far-lamp       # optional: a mains-powered device far from the lights\n\n", "", 1)
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if !strings.Contains(got, "#   player:") {
		t.Error("the commented block above canary must stay")
	}
	f = edit(t, "mqtt: {}\n")
	if err := f.SetBlock("canary", "canary: far-lamp"); err != nil {
		t.Fatal(err)
	}
	if got := string(f.Bytes()); got != "mqtt: {}\n\ncanary: far-lamp\n" {
		t.Errorf("appended: %q", got)
	}
	f = edit(t, "fixtures: [\n  a,\n  b\n]\n  # about canary\ncanary: x\n")
	if err := f.SetBlock("fixtures", "fixtures: []"); err != nil {
		t.Fatal(err)
	}
	if got := string(f.Bytes()); got != "fixtures: []\n  # about canary\ncanary: x\n" {
		t.Errorf("bracket on its own line: %q", got)
	}
}
