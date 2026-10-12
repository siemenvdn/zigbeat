package ma

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/music"
)

func TestParsePalette(t *testing.T) {
	for _, c := range []struct {
		payload string
		want    []music.RGB
	}{
		{`{"background_dark":[20,28,56],"background_light":[200,205,220],"primary":[32,45,89],"accent":[219,128,52],"on_dark":[240,220,200],"on_light":[60,30,20]}`,
			[]music.RGB{{32, 45, 89}, {219, 128, 52}, {240, 220, 200}, {60, 30, 20}, {20, 28, 56}, {200, 205, 220}}},
		{`{"primary":[180,30,40],"accent":null,"on_dark":[180,30,40],"background_dark":[120,20,25]}`,
			[]music.RGB{{180, 30, 40}, {120, 20, 25}}},
		{`{"primary":null,"accent":null,"on_dark":null,"on_light":null,"background_dark":null,"background_light":null}`, nil},
		{`{"primary":[300,0,0],"accent":[1,2],"on_dark":"red","on_light":[1.5,2,3],"background_dark":[9,9,9]}`, []music.RGB{{9, 9, 9}}},
		{`{"primary":[1,2,3],"sparkle":[4,5,6]}`, []music.RGB{{1, 2, 3}}},
		{`[]`, nil},
	} {
		if got := parsePalette(json.RawMessage(c.payload)); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.payload, got, c.want)
		}
	}
}
