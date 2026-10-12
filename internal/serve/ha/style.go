package ha

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/siemenvdn/zigbeat/internal/config"
)

// The style topics. Home Assistant sets each setting on its own topic,
// retained, so the broker hands the choices to serve on every connect.
const (
	stylePrefix = "zigbeat/style/"
	styleSuffix = "/set"
	StyleFilter = stylePrefix + "+" + styleSuffix
)

// StyleTopic is the command topic of a setting, named after its key in the config.
func StyleTopic(key string) string { return stylePrefix + key + styleSuffix }

// StyleKey is the setting a style topic sets.
func StyleKey(topic string) string {
	return strings.TrimSuffix(strings.TrimPrefix(topic, stylePrefix), styleSuffix)
}

// setting is one part of the style that Home Assistant sets.
type setting struct {
	key       string         // its key in the config: the topic level and the state key
	component map[string]any // its entity, without what every setting shares
	set       func(c *config.Config, payload string, base config.Config) error
	value     func(c config.Config) any // as the config writes it
}

// field makes a setting of the config value at points to, parsed by parse. An
// empty payload is no choice: the value comes from base, the config.
func field[T any](key string, component map[string]any, at func(*config.Config) *T, parse func(string) (T, error)) setting {
	return setting{key: key, component: component,
		set: func(c *config.Config, payload string, base config.Config) error {
			if payload == "" {
				*at(c) = *at(&base)
				return nil
			}
			v, err := parse(payload)
			if err == nil {
				*at(c) = v
			}
			return err
		},
		value: func(c config.Config) any { return *at(&c) },
	}
}

var settings = []setting{
	field("album_palette", map[string]any{"platform": "switch", "name": "Album colours", "icon": "mdi:palette"},
		func(c *config.Config) *bool { return &c.Effects.AlbumPalette }, onOff),
	field("album_colors", slider("Album colour count", "mdi:palette-swatch", config.MinAlbumColors, config.MaxAlbumColors, ""),
		func(c *config.Config) *int { return &c.Effects.AlbumColors }, whole),
	field("variant", map[string]any{"platform": "select", "name": "Variant", "icon": "mdi:shuffle-variant", "options": config.Variants},
		func(c *config.Config) *string { return &c.Effects.Variant }, word),
	field("brightness", slider("Brightness", "mdi:brightness-6", config.MinPercent, config.MaxPercent, "%"),
		func(c *config.Config) *int { return &c.Effects.Brightness }, whole),
	field("pace", map[string]any{"platform": "select", "name": "Pace", "icon": "mdi:metronome", "options": paceLabels()},
		func(c *config.Config) *int { return &c.Effects.Pace }, pace),
	field("breathe_variant", map[string]any{"platform": "select", "name": "Breathe variant", "icon": "mdi:lightbulb-on-outline", "options": config.BreatheVariants},
		func(c *config.Config) *string { return &c.Effects.BreatheVariant }, word),
	field("breathe_low", slider("Breathe low", "mdi:brightness-4", config.MinPercent, config.MaxPercent, "%"),
		func(c *config.Config) *int { return &c.Effects.BreatheRange[0] }, whole),
	field("breathe_high", slider("Breathe high", "mdi:brightness-7", config.MinPercent, config.MaxPercent, "%"),
		func(c *config.Config) *int { return &c.Effects.BreatheRange[1] }, whole),
	field("audio_offset_ms", map[string]any{"platform": "number", "name": "Audio offset", "icon": "mdi:av-timer",
		"min": config.MinOffsetMS, "max": config.MaxOffsetMS, "step": config.OffsetStepMS, "mode": "box", "unit_of_measurement": "ms"},
		func(c *config.Config) *int { return &c.Budget.AudioOffsetMS }, whole),
}

// slider is a number entity on a slider, in whole steps.
func slider(name, icon string, lo, hi float64, unit string) map[string]any {
	c := map[string]any{"platform": "number", "name": name, "icon": icon, "min": lo, "max": hi, "step": 1, "mode": "slider"}
	if unit != "" {
		c["unit_of_measurement"] = unit
	}
	return c
}

func find(key string) (setting, bool) {
	for _, s := range settings {
		if s.key == key {
			return s, true
		}
	}
	return setting{}, false
}

// Choose returns c with setting key set from Home Assistant's payload, checked
// like the config; on an error, c as it was. An empty payload wipes the
// choice: the key takes its value from base, the config. A setting that is
// gone says how to wipe its kept choice; the wipe itself changes nothing.
func Choose(c, base config.Config, key, payload string) (config.Config, error) {
	s, ok := find(key)
	switch {
	case !ok && payload == "":
		return c, nil
	case !ok:
		return c, fmt.Errorf("no such setting; wipe it with: mosquitto_pub -r -n -t %s", StyleTopic(key))
	}
	out := c
	if err := s.set(&out, payload, base); err != nil {
		return c, err
	}
	if err := errors.Join(out.Effects.Validate(), out.Budget.CheckOffset()); err != nil {
		return c, err
	}
	return out, nil
}

// Value is setting key of c as the config writes it, such as 2 or false; nil
// for a key that is no setting.
func Value(c config.Config, key string) any {
	if s, ok := find(key); ok {
		return s.value(c)
	}
	return nil
}

// word takes a select's option as it is; Validate checks it.
func word(p string) (string, error) { return p, nil }

func onOff(p string) (bool, error) {
	switch p {
	case "ON":
		return true, nil
	case "OFF":
		return false, nil
	}
	return false, errors.New("not ON or OFF")
}

func whole(p string) (int, error) {
	n, err := strconv.Atoi(p)
	if err != nil {
		return 0, errors.New("not a whole number")
	}
	return n, nil
}
