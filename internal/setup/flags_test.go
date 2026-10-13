package setup

import (
	"path/filepath"
	"testing"
)

// TestPasswordHintPrompts.
func TestPasswordHintPrompts(t *testing.T) {
	t.Setenv("ZIGBEAT_MQTT_PASSWORD", "")
	o, err := parseFlags([]string{"-config", filepath.Join(t.TempDir(), "config.yaml")})
	if err == nil {
		err = o.load()
	}
	want := "$ZIGBEAT_MQTT_PASSWORD is not set: read -rsp 'ZIGBEAT_MQTT_PASSWORD: ' ZIGBEAT_MQTT_PASSWORD && echo && export ZIGBEAT_MQTT_PASSWORD"
	if err == nil || err.Error() != want {
		t.Errorf("got %v", err)
	}
}
