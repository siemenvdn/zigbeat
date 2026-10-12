package ha

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
)

func defaults() config.Effects {
	e := config.DefaultEffects
	e.Palette = config.DefaultPalette
	return e
}

func TestStyleEntities(t *testing.T) {
	cs := decode(t, Discovery("v1"))["components"].(map[string]any)
	for _, key := range []string{"album_palette", "album_colors", "variant", "brightness", "cadence"} {
		c := cs[key].(map[string]any)
		topic := StyleTopic(key)
		if c["command_topic"] != topic || c["retain"] != true || strings.Replace(StyleFilter, "+", key, 1) != topic || StyleKey(topic) != key {
			t.Errorf("%s: %v", key, c)
		}
	}
	for key, want := range map[string][2]float64{
		"album_colors": {config.MinAlbumColors, config.MaxAlbumColors},
		"brightness":   {config.MinPercent, config.MaxPercent},
		"cadence":      {config.MinCadence.Seconds(), config.MaxCadence.Seconds()},
	} {
		c := cs[key].(map[string]any)
		if c["platform"] != "number" || c["min"] != want[0] || c["max"] != want[1] || c["mode"] != "slider" {
			t.Errorf("%s: %v, want %v", key, c, want)
		}
	}
	var opts []string
	for _, o := range cs["variant"].(map[string]any)["options"].([]any) {
		opts = append(opts, o.(string))
	}
	if !slices.Equal(opts, config.Variants) {
		t.Errorf("variant options %v", opts)
	}
}

func TestChoose(t *testing.T) {
	base := defaults()
	cur := base
	cur.Brightness = 40
	for _, c := range []struct {
		key, payload string
		want         string // the value in config form, or the error
	}{
		{"album_palette", "OFF", "false"},
		{"album_palette", "off", "not ON or OFF"},
		{"album_colors", "5", "5"},
		{"album_colors", "7", "effects.album_colors must be 2 to 6"},
		{"variant", "swap", "swap"},
		{"variant", "random", "effects.variant must be rotation, walk, shuffle, swap or build"},
		{"brightness", "100", "100"},
		{"brightness", "150", "effects.brightness must be 1 to 100"},
		{"brightness", "1e3", "not a whole number"},
		{"brightness", "", "60"},
		{"cadence", "2.5", "2.5s"},
		{"cadence", "16", "16s"},
		{"cadence", "17", "effects.cadence must be 1s to 16s"},
		{"cadence", "NaN", "not a number of seconds"},
		{"cadence", "Inf", "not a number of seconds"},
		{"cadence", "1e3", "not a number of seconds"},
		{"speed", "3", "no such setting"},
	} {
		got, err := Choose(cur, base, c.key, c.payload)
		if err != nil {
			if err.Error() != c.want || StyleOf(got) != StyleOf(cur) {
				t.Errorf("%s %q: %v, style %+v", c.key, c.payload, err, StyleOf(got))
			}
			continue
		}
		if v := fmt.Sprint(Value(got, c.key)); v != c.want {
			t.Errorf("%s %q: %s, want %s", c.key, c.payload, v, c.want)
		}
	}
}

func TestStyleInTheState(t *testing.T) {
	e := defaults()
	e.Cadence = 2500 * time.Millisecond
	st := Off
	st.Style = StyleOf(e)
	m := decode(t, st.JSON())
	want := map[string]any{"show": "OFF", "album_palette": "ON", "album_colors": 3.0, "variant": "rotation", "brightness": 60.0, "cadence": 2.5}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v, want %v", k, m[k], v)
		}
	}
}
