package ha

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/config"
)

func defaults() config.Config {
	e := config.DefaultEffects
	e.Palette = config.DefaultPalette
	return config.Config{Effects: e}
}

func TestStyleEntities(t *testing.T) {
	cs := decode(t, Discovery("v1"))["components"].(map[string]any)
	for _, key := range []string{"album_palette", "album_colors", "variant", "brightness", "pace", "breathe_variant", "breathe_low", "breathe_high", "audio_offset_ms"} {
		c := cs[key].(map[string]any)
		topic := StyleTopic(key)
		if c["command_topic"] != topic || c["retain"] != true || strings.Replace(StyleFilter, "+", key, 1) != topic || StyleKey(topic) != key {
			t.Errorf("%s: %v", key, c)
		}
	}
	for key, want := range map[string][2]float64{
		"album_colors": {config.MinAlbumColors, config.MaxAlbumColors},
		"brightness":   {config.MinPercent, config.MaxPercent},
		"breathe_low":  {config.MinPercent, config.MaxPercent},
		"breathe_high": {config.MinPercent, config.MaxPercent},
	} {
		c := cs[key].(map[string]any)
		if c["platform"] != "number" || c["min"] != want[0] || c["max"] != want[1] || c["mode"] != "slider" {
			t.Errorf("%s: %v, want %v", key, c, want)
		}
	}
	if c := cs["audio_offset_ms"].(map[string]any); c["platform"] != "number" || c["min"] != float64(config.MinOffsetMS) ||
		c["max"] != float64(config.MaxOffsetMS) || c["step"] != float64(config.OffsetStepMS) || c["mode"] != "box" || c["unit_of_measurement"] != "ms" {
		t.Errorf("audio_offset_ms: %v", c)
	}
	for key, want := range map[string][]string{
		"variant":         config.Variants,
		"pace":            {"every beat", "every 2 beats", "every bar", "every 2 bars", "every 4 bars"},
		"breathe_variant": config.BreatheVariants,
	} {
		c := cs[key].(map[string]any)
		var opts []string
		for _, o := range c["options"].([]any) {
			opts = append(opts, o.(string))
		}
		if c["platform"] != "select" || !slices.Equal(opts, want) {
			t.Errorf("%s options %v, want %v", key, opts, want)
		}
	}
	if stub := cs["cadence"].(map[string]any); len(stub) != 1 || stub["platform"] != "number" {
		t.Errorf("the old pace slider is removed with its platform only: %v", stub)
	}
}

func TestChoose(t *testing.T) {
	base := defaults()
	cur := base
	cur.Effects.Brightness = 40
	cur.Budget.AudioOffsetMS = 150
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
		{"pace", "every bar", "4"},
		{"pace", "every 3 beats", "not a pace"},
		{"pace", "4", "not a pace"},
		{"pace", "", "2"},
		{"breathe_variant", "flip", "flip"},
		{"breathe_variant", "breathe", "effects.breathe_variant must be rotation, pulse or flip"},
		{"breathe_low", "20", "20"},
		{"breathe_high", "50", "50"},
		{"breathe_low", "100", "effects.breathe_range must be [low, high] within 1 to 100 with low below high"},
		{"breathe_high", "5", "effects.breathe_range must be [low, high] within 1 to 100 with low below high"},
		{"audio_offset_ms", "180", "180"},
		{"audio_offset_ms", "-60", "-60"},
		{"audio_offset_ms", "2500", "budget.audio_offset_ms must be -500 to 2000"},
		{"audio_offset_ms", "180.5", "not a whole number"},
		{"audio_offset_ms", "", "0"},
		{"speed", "3", "no such setting; wipe it with: mosquitto_pub -r -n -t zigbeat/style/speed/set"},
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

// TestWipeOfAnUnknownSettingIsSilent: wiping a kept choice of a setting that is
// gone, such as cadence, arrives as an empty command and changes nothing.
func TestWipeOfAnUnknownSettingIsSilent(t *testing.T) {
	e := defaults()
	got, err := Choose(e, e, "cadence", "")
	if err != nil || StyleOf(got) != StyleOf(e) {
		t.Errorf("%v, %+v", err, StyleOf(got))
	}
}

func TestValueOfAnUnknownKey(t *testing.T) {
	if v := Value(defaults(), "speed"); v != nil {
		t.Errorf("got %v", v)
	}
}

func TestStyleInTheState(t *testing.T) {
	c := defaults()
	c.Effects.Pace = 4
	c.Budget.AudioOffsetMS = 120
	st := Off
	st.Style = StyleOf(c)
	m := decode(t, st.JSON())
	want := map[string]any{"show": "OFF", "album_palette": "ON", "album_colors": 3.0, "variant": "rotation", "brightness": 60.0, "pace": "every bar",
		"breathe_variant": "rotation", "breathe_low": 10.0, "breathe_high": 100.0, "audio_offset_ms": 120.0}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v, want %v", k, m[k], v)
		}
	}
}
