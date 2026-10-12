package show

import (
	"testing"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

// TestNoMomentSendsMoreThanThree is the hard rule: at most flow.MaxPerMoment
// colour messages per moment, whatever the fixtures, the mode or the backoff,
// and no kick from the calm level on.
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
		for backoff := 0; backoff < flow.PauseLevel; backoff++ {
			p := NewPlanner(c)
			p.Start(t0)
			for i := 0; i < 200; i++ {
				b := moment(4 * i)
				colour := 0
				for _, m := range p.Beat(b, backoff) {
					if _, ok := m.Payload["color"]; !ok || !m.Land.Equal(b.At) {
						continue
					}
					colour++
					if _, kick := m.Payload["brightness"]; kick && backoff >= flow.CalmLevel {
						t.Fatalf("%v, backoff %d: a kick at the calm level", c.Fixtures, backoff)
					}
				}
				if colour > flow.MaxPerMoment {
					t.Fatalf("%v, backoff %d, moment %d: %d colour messages", c.Fixtures, backoff, i, colour)
				}
			}
		}
	}
}
