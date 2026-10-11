package ha

import (
	"encoding/json"
	"math"

	"github.com/siemenvdn/zigbeat/internal/run/engine"
)

// Statuses are the values of the status sensor.
var Statuses = []string{"off", "starting", "running", "paused", "stopping"}

// State is the payload of zigbeat/state. The numbers are nil while the show
// does not play, which Home Assistant shows as unknown.
type State struct {
	Show       string   `json:"show"`
	Status     string   `json:"status"`
	FramesPerS *float64 `json:"frames_per_s"`
	LatencyMS  *int64   `json:"latency_ms"`
	Backoff    *int     `json:"backoff"`
	Lights     *int     `json:"lights"`
	LightsAway *int     `json:"lights_away"`
}

// The states while the show does not play.
var (
	Off      = State{Show: "OFF", Status: "off"}
	Starting = State{Show: "ON", Status: "starting"}
	Stopping = State{Show: "OFF", Status: "stopping"}
)

// Playing turns the show's stats into a state; total is the number of lights in the config.
func Playing(st engine.Stats, total int) State {
	status := "running"
	if st.Paused() {
		status = "paused"
	}
	fps := math.Round(st.FramesPerSec*10) / 10
	ms := st.P50.Milliseconds()
	away := total - st.Lights
	return State{Show: "ON", Status: status, FramesPerS: &fps, LatencyMS: &ms, Backoff: &st.Backoff, Lights: &st.Lights, LightsAway: &away}
}

// JSON is the payload.
func (s State) JSON() []byte {
	b, _ := json.Marshal(s)
	return b
}
