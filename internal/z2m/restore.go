package z2m

import "strings"

// SnapshotQuery asks a light for every field RestorePayload needs.
var SnapshotQuery = map[string]string{"state": "", "brightness": "", "color": "", "color_temp": ""}

// RestorePayloads turns a saved state back into set payloads, to send in order.
// A light that was off first gets its level and colour back, so it does not
// come on in the show's colour next time.
func RestorePayloads(s State) []map[string]any {
	on := onPayload(s)
	if !strings.EqualFold(s.State, "OFF") {
		return []map[string]any{on}
	}
	off := map[string]any{"state": "OFF", "transition": 0}
	if len(on) == 2 {
		return []map[string]any{off}
	}
	return []map[string]any{on, off}
}

func onPayload(s State) map[string]any {
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
