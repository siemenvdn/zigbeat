package config

import (
	"errors"
	"fmt"
	"net/url"
)

// MusicAssistant is the beat source: the visualizer relay on Music Assistant's webserver.
type MusicAssistant struct {
	URL      string `yaml:"url"`
	TokenEnv string `yaml:"token_env"`
	Player   string `yaml:"player"` // MA player id; empty follows whichever player is playing
	Token    string `yaml:"-"`      // read from TokenEnv
}

func (m *MusicAssistant) validate() error {
	u, err := url.Parse(m.URL)
	switch {
	case err == nil && u.Scheme == "wss":
		return errors.New("music_assistant.url: wss:// is not supported yet, use ws://")
	case err != nil || u.Scheme != "ws" || u.Host == "" || u.Path != "" && u.Path != "/" || u.RawQuery != "":
		return errors.New("music_assistant.url must be ws://host:port")
	case m.TokenEnv == "":
		return errors.New("music_assistant.token_env is required: the environment variable with a long-lived token from Music Assistant")
	case m.Token == "":
		return fmt.Errorf("music_assistant.token_env: $%s is not set", m.TokenEnv)
	}
	return nil
}
