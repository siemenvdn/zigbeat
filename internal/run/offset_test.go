package run

import (
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

func TestReadOffsets(t *testing.T) {
	out := make(chan time.Duration)
	go readOffsets(strings.NewReader("+\n\nnonsense\n180\n"), 0, out)
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
