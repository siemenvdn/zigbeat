package ma

import (
	"context"

	"github.com/siemenvdn/zigbeat/internal/music"
)

// track sends the properties of the track once its burst of beats is complete,
// and again when they changed. It returns false when ctx ended the wait.
func (s *session) track(ctx context.Context) bool {
	t := music.Track{BPM: s.g.bpm()}
	if !s.announced || s.sent != nil && s.sent.BPM == t.BPM {
		return true
	}
	select {
	case s.out <- t:
		s.sent = &t
		return true
	case <-ctx.Done():
		return false
	}
}
