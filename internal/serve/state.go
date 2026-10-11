package serve

import (
	"encoding/json"
	"math"

	"github.com/siemenvdn/zigbeat/internal/run/engine"
)

// statuses are the values of the status sensor.
var statuses = []string{"off", "starting", "running", "paused", "stopping"}

// state is the payload of zigbeat/state. The numbers are nil while the show
// does not play, which Home Assistant shows as unknown.
type state struct {
	Show       string   `json:"show"`
	Status     string   `json:"status"`
	FramesPerS *float64 `json:"frames_per_s"`
	LatencyMS  *int64   `json:"latency_ms"`
	Backoff    *int     `json:"backoff"`
	Lights     *int     `json:"lights"`
	LightsAway *int     `json:"lights_away"`
}

// playing turns the show's stats into a state; total is the number of lights in the config.
func playing(st engine.Stats, total int) state {
	status := "running"
	if st.Paused() {
		status = "paused"
	}
	fps := math.Round(st.FramesPerSec*10) / 10
	ms := st.P50.Milliseconds()
	away := total - st.Lights
	return state{Show: "ON", Status: status, FramesPerS: &fps, LatencyMS: &ms, Backoff: &st.Backoff, Lights: &st.Lights, LightsAway: &away}
}

func (s state) json() []byte {
	b, _ := json.Marshal(s)
	return b
}
