package probe

import (
	"fmt"
	"testing"
)

func TestNextRate(t *testing.T) {
	list := []float64{0.5, 12}
	var got []float64
	for i, r := 0, list[0]; r <= maxRate; {
		got = append(got, r)
		i++
		r = nextRate(list, i, r)
	}
	want := "[0.5 12 15 19 24 30 38 48 60 75 94]"
	if fmt.Sprint(got) != want {
		t.Errorf("got %v, want %s", got, want)
	}
	if r := nextRate(nil, 0, 0.5); r != 1.5 {
		t.Errorf("small rates must still climb, got %g", r)
	}
}
