package config

import (
	"strings"
	"testing"
)

const minimal = "budget: {max_rate: 10, latency_ms: 50}\nfixtures: [{name: p, role: color, lights: [a]}]\n"

func TestMusicAssistant(t *testing.T) {
	c, err := Parse([]byte(minimal+"music_assistant: {url: 'ws://ma:8095', token_env: MA, player: kitchen}"), env(map[string]string{"MA": "tok"}), ForShow)
	if err != nil {
		t.Fatal(err)
	}
	if m := c.MusicAssistant; m == nil || m.URL != "ws://ma:8095" || m.Token != "tok" || m.Player != "kitchen" {
		t.Errorf("music_assistant: %+v", m)
	}
	if c, err := Parse([]byte(minimal), env(nil), ForShow); err != nil || c.MusicAssistant != nil {
		t.Errorf("without the block: %+v, %v", c.MusicAssistant, err)
	}
}

func TestMusicAssistantRejects(t *testing.T) {
	cases := []struct{ name, block, want string }{
		{"url empty", "{token_env: MA}", "must be ws://host:port"},
		{"http", "{url: 'http://ma:8095', token_env: MA}", "must be ws://host:port"},
		{"no host", "{url: 'ws://', token_env: MA}", "must be ws://host:port"},
		{"a path", "{url: 'ws://ma:8095/milkdrop_visualizer', token_env: MA}", "must be ws://host:port"},
		{"wss", "{url: 'wss://ma:8095', token_env: MA}", "wss:// is not supported yet"},
		{"no token_env", "{url: 'ws://ma:8095'}", "token_env is required"},
		{"token not set", "{url: 'ws://ma:8095', token_env: UNSET}", "$UNSET is not set"},
		{"unknown key", "{url: 'ws://ma:8095', token_env: MA, port: 1}", "port"},
	}
	for _, c := range cases {
		_, err := Parse([]byte(minimal+"music_assistant: "+c.block), env(map[string]string{"MA": "tok"}), ForShow)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want %q", c.name, err, c.want)
		}
	}
}

func TestWithoutMQTT(t *testing.T) {
	cfg := []byte("mqtt: {password_env: NOPE}\n" + minimal + "music_assistant: {url: 'ws://ma:8095', token_env: MA}")
	getenv := env(map[string]string{"MA": "tok"})
	if _, err := Parse(cfg, getenv, ForShow); err == nil || !strings.Contains(err.Error(), "$NOPE is not set") {
		t.Errorf("run and serve need the MQTT password: %v", err)
	}
	if c, err := Parse(cfg, getenv, ForBeats); err != nil || c.MusicAssistant.Token != "tok" {
		t.Errorf("zigbeat beats never connects to MQTT: %+v, %v", c.MusicAssistant, err)
	}
}
