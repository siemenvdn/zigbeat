package ha

import (
	"encoding/json"
	"math"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run/engine"
)

// Statuses are the values of the status sensor.
var Statuses = []string{"off", "starting", "running", "paused", "stopping"}

// State is the payload of zigbeat/state.
type State struct {
	Show       string   `json:"show"`
	Status     string   `json:"status"`
	FramesPerS *float64 `json:"frames_per_s"`
	LatencyMS  *int64   `json:"latency_ms"`
	Backoff    *int     `json:"backoff"`
	Lights     *int     `json:"lights"`
	LightsAway *int     `json:"lights_away"`
	Style
}

// Style is the part of the state that Home Assistant sets, under the keys of the config.
type Style struct {
	AlbumPalette   string                `json:"album_palette"`
	AlbumColors    int                   `json:"album_colors"`
	Variant        config.Variant        `json:"variant"`
	Brightness     int                   `json:"brightness"`
	Pace           string                `json:"pace"`
	BreatheVariant config.BreatheVariant `json:"breathe_variant"`
	BreatheLow     int                   `json:"breathe_low"`
	BreatheHigh    int                   `json:"breathe_high"`
	FollowEnergy   string                `json:"follow_energy"`
	AudioOffsetMS  int                   `json:"audio_offset_ms"`
}

// StyleOf is the style of c for the state.
func StyleOf(c config.Config) Style {
	e := c.Effects
	return Style{AlbumPalette: onOffOf(e.AlbumPalette), AlbumColors: e.AlbumColors, Variant: e.Variant, Brightness: e.Brightness, Pace: paceLabel(e.Pace),
		BreatheVariant: e.BreatheVariant, BreatheLow: e.BreatheRange[0], BreatheHigh: e.BreatheRange[1], FollowEnergy: onOffOf(e.FollowEnergy),
		AudioOffsetMS: c.Budget.AudioOffsetMS}
}

func onOffOf(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}

const fpsDecimals = 1

// The states while the show does not play.
var (
	Off      = State{Show: "OFF", Status: "off"}
	Starting = State{Show: "ON", Status: "starting"}
	Stopping = State{Show: "OFF", Status: "stopping"}
)

// Playing turns the show's stats into a state.
func Playing(st engine.Stats, total int) State {
	status := "running"
	if st.Paused() {
		status = "paused"
	}
	scale := math.Pow10(fpsDecimals)
	fps := math.Round(st.FramesPerSec*scale) / scale
	ms := st.P50.Milliseconds()
	away := total - st.Lights
	return State{Show: "ON", Status: status, FramesPerS: &fps, LatencyMS: &ms, Backoff: &st.Backoff, Lights: &st.Lights, LightsAway: &away}
}

// JSON is the payload.
func (s State) JSON() []byte {
	b, _ := json.Marshal(s)
	return b
}
