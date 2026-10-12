package ha

import (
	"errors"
	"strconv"
	"strings"
	"time"

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
	key       string         // its key under effects: the topic level and the state key
	component map[string]any // its entity, without what every setting shares
	set       func(e *config.Effects, payload string, base config.Effects) error
	value     func(e config.Effects) any // as the config writes it
}

// field makes a setting of the effect at points to, parsed by parse. An empty
// payload is no choice: the value comes from base, the config.
func field[T any](key string, component map[string]any, at func(*config.Effects) *T, parse func(string) (T, error)) setting {
	return setting{key: key, component: component,
		set: func(e *config.Effects, payload string, base config.Effects) error {
			if payload == "" {
				*at(e) = *at(&base)
				return nil
			}
			v, err := parse(payload)
			if err == nil {
				*at(e) = v
			}
			return err
		},
		value: func(e config.Effects) any { return *at(&e) },
	}
}

var settings = []setting{
	field("album_palette", map[string]any{"platform": "switch", "name": "Album colours", "icon": "mdi:palette"},
		func(e *config.Effects) *bool { return &e.AlbumPalette }, onOff),
	field("album_colors", slider("Album colour count", "mdi:palette-swatch", config.MinAlbumColors, config.MaxAlbumColors, ""),
		func(e *config.Effects) *int { return &e.AlbumColors }, whole),
	field("variant", map[string]any{"platform": "select", "name": "Variant", "icon": "mdi:shuffle-variant", "options": config.Variants},
		func(e *config.Effects) *string { return &e.Variant }, func(p string) (string, error) { return p, nil }),
	field("brightness", slider("Brightness", "mdi:brightness-6", config.MinPercent, config.MaxPercent, "%"),
		func(e *config.Effects) *int { return &e.Brightness }, whole),
	field("cadence", slider("Pace", "mdi:metronome", config.MinCadence.Seconds(), config.MaxCadence.Seconds(), "s"),
		func(e *config.Effects) *time.Duration { return &e.Cadence }, seconds),
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

// Choose returns e with setting key set from Home Assistant's payload, checked
// like the config; on an error, e as it was. An empty payload wipes the
// choice: the key takes its value from base, the config.
func Choose(e, base config.Effects, key, payload string) (config.Effects, error) {
	s, ok := find(key)
	if !ok {
		return e, errors.New("no such setting")
	}
	out := e
	if err := s.set(&out, payload, base); err != nil {
		return e, err
	}
	if err := out.Validate(); err != nil {
		return e, err
	}
	return out, nil
}

// Value is setting key of e as the config writes it, such as 8s or false.
func Value(e config.Effects, key string) any {
	s, _ := find(key)
	return s.value(e)
}

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

// seconds reads Home Assistant's seconds, such as 2 or 2.5. As a duration it
// refuses NaN, Inf, exponents and overflow.
func seconds(p string) (time.Duration, error) {
	d, err := time.ParseDuration(p + "s")
	if err != nil {
		return 0, errors.New("not a number of seconds")
	}
	return d, nil
}
