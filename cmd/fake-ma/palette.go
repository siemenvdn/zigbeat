package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
)

// palette holds the fields of MA's MediaItemPalette in its order; nil is null.
type palette struct {
	BackgroundDark  *[3]int `json:"background_dark"`
	BackgroundLight *[3]int `json:"background_light"`
	Primary         *[3]int `json:"primary"`
	Accent          *[3]int `json:"accent"`
	OnDark          *[3]int `json:"on_dark"`
	OnLight         *[3]int `json:"on_light"`
}

// noArt is the palette of a track without artwork, and of one MA is still looking up.
var noArt = palette{}

// palettes go round per track: a navy cover with an orange title, no artwork, one red.
var palettes = []palette{
	{BackgroundDark: &[3]int{20, 28, 56}, BackgroundLight: &[3]int{200, 205, 220}, Primary: &[3]int{32, 45, 89},
		Accent: &[3]int{219, 128, 52}, OnDark: &[3]int{240, 220, 200}, OnLight: &[3]int{60, 30, 20}},
	noArt,
	{BackgroundDark: &[3]int{120, 20, 25}, Primary: &[3]int{180, 30, 40}, OnDark: &[3]int{250, 210, 210}},
}

func colorMessage(p palette) []byte {
	b, _ := json.Marshal(struct {
		Type    string  `json:"type"`
		Payload palette `json:"payload"`
	}{"color", p})
	return b
}

// cover starts a track's artwork as MA does: all null while it looks up the
// palette, the palette itself after delay.
func (tl *timeline) cover(p palette, delay time.Duration) {
	tl.color(noArt)
	if p != noArt {
		time.AfterFunc(delay, func() { tl.color(p) })
	}
}

// color sends a palette when it differs from the last one, as MA's tap does.
func (tl *timeline) color(p palette) {
	m := colorMessage(p)
	tl.mu.Lock()
	same := string(m) == string(tl.lastColor)
	tl.lastColor = m
	tl.mu.Unlock()
	if same {
		return
	}
	if tl.log {
		fmt.Printf("color %s %s\n", time.Now().Format(cli.ClockMS), m)
	}
	tl.fan(m, true)
}
