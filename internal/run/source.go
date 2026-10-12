package run

import (
	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/beat/ma"
	"github.com/siemenvdn/zigbeat/internal/config"
)

// Source is the show's beat source: Music Assistant when the config has it and
// bpm is 0, else a metronome at bpm, or at beat.DefaultBPM when bpm is 0.
func Source(cfg config.Config, bpm float64) beat.Source {
	if m := cfg.MusicAssistant; m != nil && bpm == 0 {
		return ma.Source{URL: m.URL, Token: m.Token, Player: m.Player}
	}
	if bpm == 0 {
		bpm = beat.DefaultBPM
	}
	return beat.Metronome{BPM: bpm, Lookahead: beat.Lookahead}
}
