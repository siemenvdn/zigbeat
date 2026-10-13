package show

import (
	"testing"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

const (
	moments = 200

	styleEvery = 10
)

// TestNoMomentSendsMoreThanThree is the hard rule.
func TestNoMomentSendsMoreThanThree(t *testing.T) {
	var configs []config.Config
	for _, mode := range []config.Mode{config.Separate, config.Together, config.Alternate} {
		for n := 1; n <= 8; n++ {
			configs = append(configs, fixtures(mode, n))
		}
		for a := 2; a <= 5; a++ {
			for b := 2; b <= 5; b++ {
				configs = append(configs, fixtures(mode, a, b))
			}
		}
	}
	for _, c := range configs {
		for _, variant := range config.Variants {
			c.Effects.Variant = variant
			for backoff := 0; backoff < flow.PauseLevel; backoff++ {
				noMoreThanThree(t, c, backoff)
			}
		}
	}
}

func noMoreThanThree(t *testing.T, c config.Config, backoff int) {
	t.Helper()
	p := NewPlanner(c)
	p.Start(t0)
	for i := 0; i < moments; i++ {
		if i%styleEvery == styleEvery-1 {
			e := c.Effects
			e.Variant = config.Variants[i/styleEvery%len(config.Variants)]
			e.AlbumColors = config.MinAlbumColors + i/styleEvery%2
			p.SetEffects(e)
		}
		b := moment(4 * i)
		colour := 0
		for _, m := range p.Beat(b, backoff >= flow.CalmLevel) {
			if _, ok := m.Payload["color"]; !ok || !m.Land.Equal(b.At) {
				continue
			}
			colour++
			if _, kick := m.Payload["brightness"]; kick && backoff >= flow.CalmLevel {
				t.Fatalf("%v, %s, backoff %d: a kick at the calm level", c.Fixtures, c.Effects.Variant, backoff)
			}
		}
		if colour > flow.MaxPerMoment {
			t.Fatalf("%v, %s, backoff %d, moment %d: %d colour messages", c.Fixtures, c.Effects.Variant, backoff, i, colour)
		}
	}
}
