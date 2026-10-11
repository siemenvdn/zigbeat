package z2m

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

// Echoes reports whether Zigbee2MQTT answers this payload with a state.
func Echoes(p map[string]any) bool {
	for _, k := range []string{"state", "brightness", "color"} {
		if _, ok := p[k]; ok {
			return true
		}
	}
	return false
}
