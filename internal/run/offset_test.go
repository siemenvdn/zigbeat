package run

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNextOffset(t *testing.T) {
	const ms = time.Millisecond
	cases := []struct {
		line string
		want time.Duration
		ok   bool
	}{
		{"+", 110 * ms, true}, {"+++", 130 * ms, true}, {"-", 90 * ms, true}, {" +-+ ", 110 * ms, true},
		{"180", 180 * ms, true}, {"-40", 60 * ms, true}, {"+20", 120 * ms, true}, {"", 100 * ms, true},
		{"2001", 100 * ms, false}, {"-601", 100 * ms, false}, {"+1901", 100 * ms, false}, {"later", 100 * ms, false},
	}
	for _, c := range cases {
		got, err := nextOffset(c.line, 100*ms)
		if got != c.want || (err == nil) != c.ok {
			t.Errorf("%q: %s, %v; want %s, ok %v", c.line, got, err, c.want, c.ok)
		}
	}
	if _, err := nextOffset("+", 2000*ms); err == nil || !strings.Contains(err.Error(), "-500 to 2000") {
		t.Errorf("past the limit: %v", err)
	}
}

func TestCalibrationReads(t *testing.T) {
	out := make(chan time.Duration)
	go newCalibration("", 0).read(strings.NewReader("+\n\nnonsense\n180\n"), out)
	for _, want := range []time.Duration{10 * time.Millisecond, 180 * time.Millisecond} {
		select {
		case got := <-out:
			if got != want {
				t.Errorf("got %s, want %s", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("no offset, want %s", want)
		}
	}
}

func TestCalibrationSaves(t *testing.T) {
	data, err := os.ReadFile("../../config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZIGBEAT_MQTT_PASSWORD", "secret")
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	c := newCalibration(path, 0)
	out := make(chan time.Duration, 4)
	c.read(strings.NewReader("+++\nsave\n+\n"), out)
	got, _ := os.ReadFile(path)
	want := strings.Replace(string(data), "  audio_offset_ms: 0   #", "  audio_offset_ms: 30  #", 1)
	if string(got) != want {
		t.Errorf("saved\n%s", got)
	}
	if hint := c.unsaved(); hint != "audio offset is 40 ms, not saved; type save during the show to keep it" {
		t.Errorf("hint %q", hint)
	}
	c.read(strings.NewReader("SAVE\n"), out)
	if hint := c.unsaved(); hint != "" {
		t.Errorf("hint after the last save: %q", hint)
	}
	c = newCalibration(filepath.Join(t.TempDir(), "missing.yaml"), 0)
	c.read(strings.NewReader("+\nsave\n"), out)
	if c.unsaved() == "" {
		t.Error("a failed save must leave the offset unsaved")
	}
}
