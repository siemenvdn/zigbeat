package run

import (
	"fmt"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/config"
)

func TestSource(t *testing.T) {
	plain := config.Config{}
	withMA := config.Config{MusicAssistant: &config.MusicAssistant{URL: "ws://ma:8095", Token: "t"}}
	cases := []struct {
		cfg  config.Config
		bpm  float64
		want string
	}{
		{plain, 0, "metronome at 120 BPM"},
		{plain, 90, "metronome at 90 BPM"},
		{withMA, 0, "Music Assistant at ws://ma:8095 (any playing player)"},
		{withMA, 100, "metronome at 100 BPM"},
	}
	for _, c := range cases {
		if got := fmt.Sprint(Source(c.cfg, c.bpm)); got != c.want {
			t.Errorf("Source(bpm %g) = %s, want %s", c.bpm, got, c.want)
		}
	}
}
