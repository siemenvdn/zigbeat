package ma

import (
	"context"
	"encoding/json"
	"math"
	"slices"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
)

// nullGrace: MA sends an all-null palette at every track change while it looks
// up the next one, from its cache in milliseconds. A null counts only after this.
var nullGrace = 10 * time.Second

// roles are the palette's fields in the order zigbeat uses them: the colours of
// the image first, then the backgrounds MA derives from primary.
var roles = []string{"primary", "accent", "on_dark", "on_light", "background_dark", "background_light"}

const channels = 3 // red, green, blue

// parsePalette reads a color@v1 payload. Nulls, repeats and fields that are not
// three numbers from 0 to 255 are left out; nil means no colour at all.
func parsePalette(raw json.RawMessage) []music.RGB {
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	var out []music.RGB
	for _, r := range roles {
		var c []int
		if json.Unmarshal(fields[r], &c) != nil || len(c) != channels {
			continue
		}
		var rgb music.RGB
		valid := true
		for i, v := range c {
			valid = valid && v >= 0 && v <= math.MaxUint8
			rgb[i] = uint8(v)
		}
		if valid && !slices.Contains(out, rgb) {
			out = append(out, rgb)
		}
	}
	return out
}

// album takes a palette from the relay: a real one at once, null only once
// nullGrace passed without a real one. It returns false when ctx ended the wait.
func (s *session) album(ctx context.Context, p []music.RGB) bool {
	if p == nil {
		if s.palette != nil {
			s.fade.Reset(nullGrace)
		}
		return true
	}
	s.fade.Stop()
	s.palette = p
	return s.track(ctx)
}
