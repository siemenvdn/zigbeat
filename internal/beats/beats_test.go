package beats

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
)

func TestPrintAtTheBeat(t *testing.T) {
	events := make(chan music.Event, 3)
	events <- music.Track{BPM: 124}
	events <- music.Beat{At: time.Now().Add(50 * time.Millisecond), Index: -1, InBar: 3}
	events <- music.Beat{At: time.Now().Add(100 * time.Millisecond), Index: 0, InBar: 0}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	print(ctx, events, 20*time.Millisecond, &out)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.HasSuffix(lines[0], "bar -1 beat 4  124.0 BPM") || !strings.HasSuffix(lines[1], "bar 0 beat 1  124.0 BPM") {
		t.Errorf("lines %q", lines)
	}
}

func TestNeedsMusicAssistant(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := "budget: {max_rate: 10, latency_ms: 50}\nfixtures: [{name: p, role: color, lights: [a]}]\n"
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"-config", path}); code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
	if code := Run([]string{"-nope"}); code != 2 {
		t.Errorf("a bad flag: exit code %d, want 2", code)
	}
}

func TestClearDropsPendingLines(t *testing.T) {
	events := make(chan music.Event, 3)
	events <- music.Beat{At: time.Now().Add(50 * time.Millisecond), Index: 1, InBar: 1}
	events <- music.Clear{}
	events <- music.Beat{At: time.Now().Add(100 * time.Millisecond), Index: 0, InBar: 0}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	print(ctx, events, 0, &out)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 || !strings.HasSuffix(lines[0], "bar 0 beat 1  120.0 BPM") {
		t.Errorf("a clear drops the beats not printed yet: %q", lines)
	}
}
