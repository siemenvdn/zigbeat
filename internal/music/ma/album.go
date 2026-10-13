package ma

import (
	"context"
	"encoding/json"
	"math"
	"slices"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
)

var nullGrace = 10 * time.Second

var roles = []string{"primary", "accent", "on_dark", "on_light", "background_dark", "background_light"}

const channels = 3

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
