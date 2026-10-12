package setup

import (
	"fmt"
	"strings"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/config"
)

func TestStem(t *testing.T) {
	for name, want := range map[string]string{
		"pendant-1": "pendant", "pendant_12": "pendant", "Living Room 2": "Living Room", "desk-lamp": "desk-lamp", "42": "42", "a.3": "a",
		"0x0017880104e45518": "0x0017880104e45518",
	} {
		if got := stem(name); got != want {
			t.Errorf("stem(%q) = %q, want %q", name, got, want)
		}
	}
}

func show(fs []config.Fixture) string {
	var parts []string
	for _, f := range fs {
		parts = append(parts, fmt.Sprintf("%s/%s/%s:%s", f.Name, f.Role, f.Mode, strings.Join(f.Lights, ",")))
	}
	return strings.Join(parts, " ")
}

func TestArrangeNew(t *testing.T) {
	chosen := []light{{"pendant-1", "color", ""}, {"pendant-2", "color", ""}, {"floor-lamp", "breathe", ""}, {"desk-lamp", "color", ""}, {"pendant-3", "breathe", ""}}
	fs, notes := arrange(nil, chosen)
	want := "pendant/color/:pendant-1,pendant-2 floor-lamp/breathe/:floor-lamp desk-lamp/color/:desk-lamp pendant-3/breathe/:pendant-3"
	if show(fs) != want || len(notes) != 0 {
		t.Errorf("got %s %v\nwant %s", show(fs), notes, want)
	}
}

func TestArrangeExisting(t *testing.T) {
	existing := []config.Fixture{
		{Name: "pendant", Role: "color", Mode: "together", Lights: []string{"pendant-1", "pendant-2", "pendant-9"}},
		{Name: "desk", Role: "color", Mode: "alternate", Lights: []string{"desk-lamp"}},
		{Name: "gone", Role: "breathe", Lights: []string{"old-lamp"}},
	}
	chosen := []light{{"pendant-1", "color", ""}, {"pendant-2", "color", ""}, {"pendant-3", "color", ""}, {"desk-lamp", "breathe", ""}, {"bed-1", "color", ""}, {"bed-2", "color", ""}, {"pendant-4", "breathe", ""}}
	fs, notes := arrange(existing, chosen)
	want := "pendant/color/together:pendant-1,pendant-2,pendant-3 desk-lamp/breathe/:desk-lamp bed/color/:bed-1,bed-2 pendant-4/breathe/:pendant-4"
	if show(fs) != want {
		t.Errorf("got  %s\nwant %s", show(fs), want)
	}
	if strings.Join(notes, ";") != "desk-lamp: no xy colour; now breathe" {
		t.Errorf("notes %v", notes)
	}
}

func TestArrangeTakenNames(t *testing.T) {
	existing := []config.Fixture{{Name: "lamp", Role: "breathe", Lights: []string{"floor"}}}
	fs, _ := arrange(existing, []light{{"floor", "breathe", ""}, {"lamp-1", "color", ""}, {"lamp-2", "color", ""}, {"lamp", "color", ""}})
	if want := "lamp/breathe/:floor lamp-2/color/:lamp-1,lamp-2,lamp"; show(fs) != want {
		t.Errorf("got  %s\nwant %s", show(fs), want)
	}
}
