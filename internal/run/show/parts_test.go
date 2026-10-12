package show

import (
	"fmt"
	"strings"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/config"
)

// fixtures returns a config whose colour fixtures have the given sizes, all in
// mode, named f0-1, f0-2, ..., and one breathe light.
func fixtures(mode string, sizes ...int) config.Config {
	c := testConfig(mode)
	var fs []config.Fixture
	for i, n := range sizes {
		f := config.Fixture{Name: fmt.Sprintf("f%d", i), Role: "color", Mode: mode}
		for j := 1; j <= n; j++ {
			f.Lights = append(f.Lights, fmt.Sprintf("f%d-%d", i, j))
		}
		fs = append(fs, f)
	}
	c.Fixtures = append(fs, c.Fixtures[len(c.Fixtures)-1]) // and lamp-4, breathing
	return c
}

// colours lists the colour each light got in ps.
func colours(ps []Message) map[string]string {
	got := map[string]string{}
	for _, m := range ps {
		if c, ok := m.Payload["color"]; ok {
			got[m.Light] = fmt.Sprint(c)
		}
	}
	return got
}

func TestLargeUnitChangesInParts(t *testing.T) {
	p := NewPlanner(fixtures("together", 5))
	first, second := p.Beat(moment(0), 0), p.Beat(moment(4), 0)
	if got := changed(first); got != "f0-1,f0-2,f0-3" {
		t.Fatalf("the first part: %q", got)
	}
	if got := changed(second); got != "f0-4,f0-5" {
		t.Fatalf("the rest at the next moment: %q", got)
	}
	a, b := colours(first), colours(second)
	if a["f0-1"] != b["f0-4"] || a["f0-1"] != b["f0-5"] {
		t.Errorf("one colour for the whole fixture: %v then %v", a, b)
	}
	if got := changed(p.Beat(moment(8), 0)); got != "f0-1,f0-2,f0-3" {
		t.Errorf("then the fixture starts again: %q", got)
	}
}

func TestSevenLightsTakeThreeMoments(t *testing.T) {
	p := NewPlanner(fixtures("together", 7))
	var got []string
	for i := 0; i < 3; i++ {
		got = append(got, changed(p.Beat(moment(4*i), 0)))
	}
	if want := "f0-1,f0-2,f0-3 f0-4,f0-5,f0-6 f0-7"; strings.Join(got, " ") != want {
		t.Errorf("parts %v, want %s", got, want)
	}
}

func TestTheChaseWaitsForTheParts(t *testing.T) {
	c := fixtures("together", 5)
	c.Fixtures = append(c.Fixtures[:1], config.Fixture{Name: "pair", Role: "color", Mode: "separate", Lights: []string{"a", "b"}}, c.Fixtures[1])
	p := NewPlanner(c)
	var got []string
	for i := 0; i < 4; i++ {
		got = append(got, changed(p.Beat(moment(4*i), 0)))
	}
	if want := "f0-1,f0-2,f0-3 f0-4,f0-5 a,b f0-1,f0-2,f0-3"; strings.Join(got, " ") != want {
		t.Errorf("moments %v, want %s", got, want)
	}
}
