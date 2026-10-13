package setup

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/mqtt"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Flag defaults for a new config: the MQTT user and variable the README suggests.
const (
	defaultUser        = "zigbeat"
	defaultPasswordEnv = "ZIGBEAT_MQTT_PASSWORD"
	noCanary           = "none"
)

type options struct {
	config       string
	lights       []string // -lights; nil asks
	canary       string   // -canary; empty asks, noCanary for none
	mqtt         config.MQTT
	existing     *config.Config // the config at -config, nil when there is none
	mqttFlagsSet []string
}

func parseFlags(args []string) (options, error) {
	var o options
	var lights string
	fs := cli.NewFlagSet("setup", "usage: zigbeat setup [-config config.yaml] [-lights a,b] [-canary name|none] [mqtt flags for a new config]",
		"Finds your lights in Zigbee2MQTT, proposes roles and fixtures, and writes the configuration. See the README.")
	fs.StringVar(&o.config, "config", config.DefaultPath, "file to write; when it exists, only fixtures and canary change")
	fs.StringVar(&lights, "lights", "", "comma-separated lights for the show; skips the question")
	fs.StringVar(&o.canary, "canary", "", "the canary, or none; skips the question")
	fs.StringVar(&o.mqtt.Broker, "broker", mqtt.DefaultBroker, "MQTT broker host:port, for a new config")
	fs.StringVar(&o.mqtt.Username, "user", defaultUser, "MQTT username, for a new config; empty for none")
	fs.StringVar(&o.mqtt.PasswordEnv, "password-env", defaultPasswordEnv, "environment variable with the MQTT password, for a new config")
	fs.StringVar(&o.mqtt.BaseTopic, "base", z2m.DefaultBaseTopic, "Zigbee2MQTT base topic, for a new config")
	if err := cli.Parse(fs, args); err != nil {
		return o, err
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "broker", "user", "password-env", "base":
			o.mqttFlagsSet = append(o.mqttFlagsSet, "-"+f.Name)
		}
	})
	o.lights = cli.Names(lights)
	if lights != "" && len(o.lights) == 0 {
		return o, errors.New("-lights needs at least one name")
	}
	if _, err := os.Stat(o.config); err == nil && len(o.mqttFlagsSet) > 0 {
		return o, fmt.Errorf("%s are for a new config; edit the mqtt block in %s instead", strings.Join(o.mqttFlagsSet, ", "), o.config)
	}
	return o, nil
}

// load reads the config at -config when it exists, and settles the MQTT
// settings: from that config, or from the flags and the environment.
func (o *options) load() error {
	if _, err := os.Stat(o.config); err == nil {
		c, err := config.Load(o.config, config.ForDraft)
		if err != nil {
			return err
		}
		o.existing, o.mqtt = &c, c.MQTT
		return nil
	}
	if o.mqtt.Username == "" {
		o.mqtt.PasswordEnv = ""
		return nil
	}
	if o.mqtt.Password = os.Getenv(o.mqtt.PasswordEnv); o.mqtt.Password == "" {
		return fmt.Errorf("$%s is not set: read -rsp '%s: ' %s && echo && export %s", o.mqtt.PasswordEnv, o.mqtt.PasswordEnv, o.mqtt.PasswordEnv, o.mqtt.PasswordEnv)
	}
	return nil
}
