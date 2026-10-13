package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func edit(t *testing.T, yaml string) *File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func example(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSave(t *testing.T) {
	f := edit(t, "budget:\n  max_rate: 4\n")
	if err := f.Set("budget.max_rate", "15", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.Save(func([]byte) error { return errors.New("no") }); err == nil {
		t.Fatal("a failing check must stop the save")
	}
	if data, _ := os.ReadFile(f.path); string(data) != "budget:\n  max_rate: 4\n" {
		t.Fatalf("the file changed: %q", data)
	}
	if err := f.Save(func([]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(f.path)
	info, _ := os.Stat(f.path)
	if string(data) != "budget:\n  max_rate: 15\n" || info.Mode().Perm() != 0o600 {
		t.Errorf("saved %q with mode %v", data, info.Mode().Perm())
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(f.path), ".config.yaml.*")); len(left) > 0 {
		t.Errorf("temporary files left: %v", left)
	}
}

func TestDraftNeedsNoBudget(t *testing.T) {
	cfg := []byte("budget: {max_rate: 0, latency_ms: 0}\nfixtures: [{name: p, role: color, lights: [a]}]\n")
	if _, err := Parse(cfg, env(nil), ForDraft); err != nil {
		t.Errorf("a draft without a budget: %v", err)
	}
	if _, err := Parse(cfg, env(nil), ForShow); err == nil || !strings.Contains(err.Error(), "budget.max_rate is required") {
		t.Errorf("run needs the budget: %v", err)
	}
}
