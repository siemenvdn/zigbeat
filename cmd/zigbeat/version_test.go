package main

import (
	"runtime/debug"
	"testing"
)

func TestChosenVersion(t *testing.T) {
	built := func(v string) *debug.BuildInfo { return &debug.BuildInfo{Main: debug.Module{Version: v}} }
	for _, c := range []struct {
		name    string
		stamped string
		info    *debug.BuildInfo
		ok      bool
		want    string
	}{
		{"stamped by a release", "v1.0.0", built("v0.9.0"), true, "v1.0.0"},
		{"go install at a tag", "", built("v1.0.0"), true, "v1.0.0"},
		{"a commit after a tag", "", built("v1.0.1-0.20261013062515-89b6e2480cf8"), true, "v1.0.1-0.20261013062515-89b6e2480cf8"},
		{"uncommitted changes", "", built("v1.0.0+dirty"), true, "v1.0.0+dirty"},
		{"go run, or no git", "", built("(devel)"), true, "dev"},
		{"no version", "", built(""), true, "dev"},
		{"no build info", "", nil, false, "dev"},
	} {
		if got := chosenVersion(c.stamped, c.info, c.ok); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
