package setup

import (
	"fmt"
	"testing"
)

func TestParseSelection(t *testing.T) {
	good := map[string]string{"1-3,7": "[0 1 2 6]", " 2 , 2,1": "[0 1]", "7": "[6]", "1-7": "[0 1 2 3 4 5 6]"}
	for in, want := range good {
		got, err := parseSelection(in, 7)
		if err != nil || fmt.Sprint(got) != want {
			t.Errorf("%q: %v %v, want %s", in, got, err, want)
		}
	}
	for _, in := range []string{"", "0", "8", "3-1", "x", "1-", "-2", "1,,x"} {
		if got, err := parseSelection(in, 7); err == nil {
			t.Errorf("%q: %v accepted", in, got)
		}
	}
}
