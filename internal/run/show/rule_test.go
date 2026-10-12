package show

import (
	"testing"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

const (
	// moments covers more than three phrases at the test config's pace.
	moments = 200
	// styleEvery: moments between style changes, as Home Assistant may send them.
	styleEvery = 10
)

// TestNoMomentSendsMoreThanThree is the hard rule: at most flow.MaxPerMoment
// colour messages per moment, whatever the variant, the fixtures, the mode or
// the backoff, also when the style changes along the way, and no kick from the
// calm level on.
func TestNoMomentSendsMoreThanThree(t *testing.T) {
	var configs []config.Config
	for _, mode := range []string{"separate", "together", "alternate"} {
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
		for _, variant := range []string{"rotation", "walk", "shuffle", "swap", "build"} {
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
		for _, m := range p.Beat(b, backoff) {
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
