package show

import (
	"strings"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/music"
)

// Covers as Music Assistant's roles could describe them, most dominant first.
var (
	navyCover = []music.RGB{{32, 45, 89}, {219, 128, 52}, {240, 220, 200}, {60, 30, 20}, {20, 28, 56}, {200, 205, 220}}
	greyPhoto = []music.RGB{{30, 30, 30}, {200, 200, 200}, {240, 240, 240}, {10, 10, 10}}
	oneRed    = []music.RGB{{180, 30, 40}, {120, 20, 25}, {250, 210, 210}}
	brownWood = []music.RGB{{110, 70, 40}, {200, 170, 130}, {60, 40, 25}}
)

func TestAlbumColors(t *testing.T) {
	for _, c := range []struct {
		name  string
		cover []music.RGB
		want  []string // for n = 2 to 6
	}{
		{"navy, orange title", navyCover, []string{
			"#406cff #ff943d",
			"#406cff #ff943d #7340ff",
			"#406cff #ff943d #7340ff #40ccff",
			"#406cff #ff943d #7340ff #40ccff #d240ff",
			"#406cff #ff943d #7340ff #40ccff #d240ff #40ffd2",
		}},
		{"one red", oneRed, []string{
			"#ff2b39 #ff872b",
			"#ff2b39 #ff872b #ff2ba3",
			"#ff2b39 #ff872b #ff2ba3 #fff12b",
			"#ff2b39 #ff872b #ff2ba3 #fff12b #f12bff",
			"#ff2b39 #ff872b #ff2ba3 #fff12b #f12bff #a3ff2b",
		}},
		{"brown wood", brownWood, []string{
			"#ff9340 #fff240",
			"#ff9340 #fff240 #ff404c",
			"#ff9340 #fff240 #ff404c #acff40",
			"#ff9340 #fff240 #ff404c #acff40 #ff40ac",
			"#ff9340 #fff240 #ff404c #acff40 #ff40ac #4dff40",
		}},
	} {
		for i, want := range c.want {
			if got := strings.Join(AlbumColors(c.cover, i+2), " "); got != want {
				t.Errorf("%s, %d colours: %s, want %s", c.name, i+2, got, want)
			}
		}
	}
	for _, cover := range [][]music.RGB{greyPhoto, nil} {
		if got := AlbumColors(cover, 3); got != nil {
			t.Errorf("no colour in %v: %v", cover, got)
		}
	}
}
