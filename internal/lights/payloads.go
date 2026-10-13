package lights

import (
	"strings"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

var query = map[string]string{"state": "", "brightness": "", "color": "", "color_temp": ""}

const queryFrames = 3

func payloads(s z2m.State) []map[string]any {
	on := onPayload(s)
	if !strings.EqualFold(s.State, "OFF") {
		return []map[string]any{on}
	}
	off := map[string]any{"state": "OFF", "transition": 0}
	if len(on) == len(off) {
		return []map[string]any{off}
	}
	return []map[string]any{on, off}
}

func onPayload(s z2m.State) map[string]any {
	p := map[string]any{"state": "ON", "transition": 0}
	if s.Brightness != nil {
		p["brightness"] = *s.Brightness
	}
	switch {
	case s.ColorMode == "color_temp" && s.ColorTemp != nil:
		p["color_temp"] = *s.ColorTemp
	case s.Color != nil:
		p["color"] = s.Color
	}
	return p
}
