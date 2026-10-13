package ma

import (
	"context"
	"slices"

	"github.com/siemenvdn/zigbeat/internal/music"
)

func (s *session) track(ctx context.Context) bool {
	t := music.Track{BPM: s.g.bpm(), Palette: s.palette}
	if !s.announced || s.sent != nil && s.sent.BPM == t.BPM && slices.Equal(s.sent.Palette, t.Palette) {
		return true
	}
	if !s.send(ctx, t) {
		return false
	}
	s.sent = &t
	return true
}
