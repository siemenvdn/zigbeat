package cli

import (
	"errors"
	"flag"
	"io"
	"testing"
)

func TestParse(t *testing.T) {
	fs := NewFlagSet("run", "usage: zigbeat run", "Runs the show.")
	fs.SetOutput(io.Discard)
	n := fs.Int("n", 1, "a number")
	if err := Parse(fs, []string{"-n", "2"}); err != nil || *n != 2 {
		t.Fatalf("n %d, err %v", *n, err)
	}
	if err := Parse(fs, []string{"-h"}); err != flag.ErrHelp {
		t.Errorf("-h: %v", err)
	}
	if err := Parse(fs, []string{"-nope"}); err != ErrUsage {
		t.Errorf("unknown flag: %v", err)
	}
}

func TestExit(t *testing.T) {
	cases := []struct {
		err  error
		code int
		stop bool
	}{
		{nil, 0, false},
		{flag.ErrHelp, 0, true},
		{ErrUsage, 2, true},
		{errors.New("-bpm must be 30 to 300"), 2, true},
	}
	for _, c := range cases {
		if code, stop := Exit(c.err); code != c.code || stop != c.stop {
			t.Errorf("Exit(%v) = %d, %v", c.err, code, stop)
		}
	}
}

func TestNames(t *testing.T) {
	if got := Names(" lamp-1, ,lamp-2,"); len(got) != 2 || got[0] != "lamp-1" || got[1] != "lamp-2" {
		t.Errorf("Names = %q", got)
	}
	if got := Names(""); got != nil {
		t.Errorf("Names of nothing = %q", got)
	}
}
