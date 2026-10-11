package z2m

import "strings"

// SnapshotQuery asks a light for every field RestorePayload needs.
var SnapshotQuery = map[string]string{"state": "", "brightness": "", "color": "", "color_temp": ""}

// RestorePayload turns a saved state back into a set payload.
func RestorePayload(s State) map[string]any {
	if strings.EqualFold(s.State, "OFF") {
		return map[string]any{"state": "OFF", "transition": 0}
	}
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

// Frames estimates the Zigbee frames Zigbee2MQTT sends for a set payload:
// state and brightness share one command; color, color_temp and brightness_move take one each.
func Frames(p map[string]any) int {
	n := 0
	_, state := p["state"]
	_, bri := p["brightness"]
	if state || bri {
		n++
	}
	for _, k := range []string{"color", "color_temp", "brightness_move"} {
		if _, ok := p[k]; ok {
			n++
		}
	}
	return n
}
