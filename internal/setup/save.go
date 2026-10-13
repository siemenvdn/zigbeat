package setup

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/siemenvdn/zigbeat/internal/config"
)

// write saves the configuration: a new one from the template, or only the
// fixtures and the canary of the existing one. Nothing is written unless it
// reads back as a valid config with exactly the chosen lights.
func (s *session) write(fixtures []config.Fixture, canary string) error {
	check := func(data []byte) error {
		c, err := config.Parse(data, os.Getenv, config.ForDraft)
		if err == nil && !slices.Equal(c.Lights(), config.LightsOf(fixtures)) {
			err = errors.New("the lights read back differ from the ones chosen")
		}
		if err != nil {
			return fmt.Errorf("not writing %s: %v", s.o.config, err)
		}
		return nil
	}
	if s.o.existing == nil {
		text := newConfig(s.o.config, s.o.mqtt, canary, fixtures, s.today)
		return config.NewFile(s.o.config, []byte(text)).Save(check)
	}
	f, err := config.OpenFile(s.o.config)
	if err == nil {
		err = f.SetBlock("fixtures", renderFixtures(fixtures))
	}
	if err == nil && canary != s.o.existing.Canary {
		err = setCanary(f, canary)
	}
	if err != nil {
		return err
	}
	return f.Save(check)
}

// setCanary replaces the canary of f, or removes it for none. Set refuses a
// quoted or empty old value, so then the whole line is replaced.
func setCanary(f *config.File, canary string) error {
	if canary == "" {
		return f.SetBlock("canary", "")
	}
	if f.Set("canary", yamlName(canary), "") == nil {
		return nil
	}
	return f.SetBlock("canary", canaryLine(canary))
}

// done says what was written and what comes next.
func (s *session) done(fixtures []config.Fixture, canary string) {
	about := "none"
	if canary != "" {
		about = canary
	}
	next := "zigbeat probe -config " + s.o.config
	if s.o.existing != nil && s.o.existing.Budget.MaxRate > 0 {
		next = "zigbeat run -config " + s.o.config
	}
	fmt.Fprintf(s.out, "wrote %s (lights: %d, fixtures: %d, canary: %s); next: %s\n", s.o.config, len(config.LightsOf(fixtures)), len(fixtures), about, next)
}
