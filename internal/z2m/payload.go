package z2m

// Frames estimates the Zigbee frames Zigbee2MQTT sends for a set payload.
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
