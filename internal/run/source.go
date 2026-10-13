package run

import (
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/music/ma"
)

// Source is the show's beat source.
func Source(cfg config.Config, bpm float64) music.Source {
	if m := cfg.MusicAssistant; m != nil && bpm == 0 {
		return ma.Source{URL: m.URL, Token: m.Token, Player: m.Player}
	}
	if bpm == 0 {
		bpm = music.DefaultBPM
	}
	return music.Metronome{BPM: bpm, Lookahead: music.Lookahead}
}
