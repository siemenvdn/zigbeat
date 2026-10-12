package ha

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/run/engine"
)

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestStateNotPlaying(t *testing.T) {
	m := decode(t, Starting.JSON())
	if m["show"] != "ON" || m["status"] != "starting" {
		t.Errorf("state %v", m)
	}
	for _, st := range []State{Off, Starting, Stopping} {
		if !slices.Contains(Statuses, st.Status) {
			t.Errorf("%s is not a status sensor option", st.Status)
		}
	}
	for _, k := range []string{"frames_per_s", "latency_ms", "backoff", "lights", "lights_away"} {
		if v, ok := m[k]; !ok || v != nil {
			t.Errorf("%s must be null, got %v (present %v)", k, v, ok)
		}
	}
}

func TestStatePlaying(t *testing.T) {
	st := engine.Stats{FramesPerSec: 4.2499, P50: 38700 * time.Microsecond, Backoff: 1, Lights: 3}
	m := decode(t, Playing(st, 4).JSON())
	want := map[string]any{"show": "ON", "status": "running", "frames_per_s": 4.2, "latency_ms": 38.0, "backoff": 1.0, "lights": 3.0, "lights_away": 1.0}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v, want %v", k, m[k], v)
		}
	}
	st.Backoff = 2
	if s := Playing(st, 4); s.Status != "paused" {
		t.Errorf("backoff 2 is paused, got %s", s.Status)
	}
}

func TestDiscovery(t *testing.T) {
	m := decode(t, Discovery("v1.2.3"))
	for _, k := range []string{"device", "origin", "components"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("discovery lacks %s", k)
		}
	}
	if m["state_topic"] != StateTopic || m["availability_topic"] != AvailTopic {
		t.Errorf("topics %v %v", m["state_topic"], m["availability_topic"])
	}
	keys := decode(t, Playing(engine.Stats{}, 1).JSON())
	ids := map[string]bool{}
	for name, raw := range m["components"].(map[string]any) {
		c := raw.(map[string]any)
		if platform, gone := removed[name]; gone {
			if len(c) != 1 || c["platform"] != platform {
				t.Errorf("%s: a removed component is its platform only: %v", name, c)
			}
			continue
		}
		id, _ := c["unique_id"].(string)
		if c["platform"] == nil || id == "" || ids[id] {
			t.Errorf("%s: platform %v, unique_id %q", name, c["platform"], id)
		}
		ids[id] = true
		tpl, _ := c["value_template"].(string)
		key := strings.TrimSuffix(strings.TrimPrefix(tpl, "{{ value_json."), " }}")
		if _, ok := keys[key]; !ok {
			t.Errorf("%s: value_template %v names no state key", name, c["value_template"])
		}
	}
	if len(ids) != 15 {
		t.Errorf("want 15 entities, got %d", len(ids))
	}
	status := m["components"].(map[string]any)["status"].(map[string]any)
	var opts []string
	for _, o := range status["options"].([]any) {
		opts = append(opts, o.(string))
	}
	if !slices.Equal(opts, Statuses) {
		t.Errorf("status options %v", opts)
	}
	if m["device"].(map[string]any)["sw_version"] != "v1.2.3" {
		t.Errorf("device %v", m["device"])
	}
}
