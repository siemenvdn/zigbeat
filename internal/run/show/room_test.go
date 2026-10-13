package show

import (
	"fmt"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

const roomBeats = 160

// TestEveryMomentFitsItsRoom.
func TestEveryMomentFitsItsRoom(t *testing.T) {
	for _, mode := range []config.Mode{config.Separate, config.Together, config.Alternate} {
		for n := 1; n <= 8; n++ {
			for _, variant := range config.Variants {
				for _, breathe := range config.BreatheVariants {
					for _, rate := range []float64{4, 15, 100} {
						for _, bpm := range []float64{90, 120, 174} {
							for backoff := range flow.PauseLevel {
								c := fixtures(mode, n)
								c.Fixtures[len(c.Fixtures)-1].Lights = []string{"lamp-4", "lamp-5"}
								c.Effects.Variant, c.Effects.BreatheVariant, c.Effects.Pace = variant, breathe, 2
								c.Budget.MaxRate = rate
								fitsItsRoom(t, c, bpm, backoff)
							}
						}
					}
				}
			}
		}
	}
}

func fitsItsRoom(t *testing.T, c config.Config, bpm float64, backoff int) {
	t.Helper()
	name := fmt.Sprintf("%v %s/%s %g/s %g BPM backoff %d", c.Fixtures, c.Effects.Variant, c.Effects.BreatheVariant, c.Budget.MaxRate, bpm, backoff)
	p := NewPlanner(c)
	p.Start(t0)
	p.Track(music.Track{BPM: bpm})
	for i := range roomBeats {
		room := p.pace.room(p.interval())
		ps := p.Beat(moment(i), backoff >= flow.CalmLevel)
		if ps == nil {
			continue
		}
		colour, white, kicks := 0, 0, 0
		for _, m := range ps {
			switch {
			case m.Payload["color"] != nil:
				colour++
				if m.Payload["brightness"] != nil {
					kicks++
				}
			case m.Light == "lamp-4" || m.Light == "lamp-5":
				if m.Land.Equal(moment(i).At) {
					white++
				}
			}
		}
		switch {
		case float64(framesOf(ps)) > room:
			t.Fatalf("%s, beat %d: %d frames in a room of %.1f", name, i, framesOf(ps), room)
		case colour > flow.MaxPerMoment || white > flow.MaxPerMoment:
			t.Fatalf("%s, beat %d: %d colour, %d white", name, i, colour, white)
		case kicks != 0 && kicks != colour:
			t.Fatalf("%s, beat %d: %d of %d colour changes kick", name, i, kicks, colour)
		case kicks != 0 && backoff >= flow.CalmLevel:
			t.Fatalf("%s, beat %d: a kick at the calm level", name, i)
		}
	}
}
