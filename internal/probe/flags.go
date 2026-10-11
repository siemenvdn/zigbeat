package probe

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
)

type config struct {
	broker, user, password, base string
	targets                      []string
	canary                       string
	rates                        []float64
	step, cooldown, baseline     time.Duration
	timeout                      time.Duration
	mode, out                    string
	yes                          bool
}

func parseFlags(args []string) (config, error) {
	var cfg config
	var targets, rates string
	fs := cli.NewFlagSet("probe", "usage: zigbeat probe -targets <names> -canary <name> [flags]",
		"Measures how many commands per second your Zigbee network handles. See the README.")
	fs.StringVar(&cfg.broker, "broker", "127.0.0.1:1883", "MQTT broker host:port")
	fs.StringVar(&cfg.user, "user", "", "MQTT username (falls back to $MQTT_USER)")
	fs.StringVar(&cfg.password, "password", "", "MQTT password (falls back to $MQTT_PASSWORD)")
	fs.StringVar(&cfg.base, "base", "zigbee2mqtt", "Zigbee2MQTT base topic")
	fs.StringVar(&targets, "targets", "", "comma-separated Zigbee2MQTT friendly names of colour lights to drive (required)")
	fs.StringVar(&cfg.canary, "canary", "", "friendly name of a mains-powered device far from the targets, used to watch the rest of the network (required)")
	fs.StringVar(&rates, "rates", "1,2,3,4,6,8,10,12", "comma-separated starting rates (cmd/s, all targets together); after these the probe climbs by 25% per step until trouble")
	fs.DurationVar(&cfg.step, "step", 20*time.Second, "duration of each rate step")
	fs.DurationVar(&cfg.cooldown, "cooldown", 10*time.Second, "pause between steps")
	fs.DurationVar(&cfg.baseline, "baseline", 20*time.Second, "canary-only measurement before the steps")
	fs.DurationVar(&cfg.timeout, "timeout", 5*time.Second, "a command or probe without answer after this counts as lost")
	fs.StringVar(&cfg.mode, "mode", "color", "command type: color, brightness or combo")
	fs.StringVar(&cfg.out, "out", "zigbeat-probe-"+time.Now().Format("20060102-150405")+".csv", "CSV file for raw samples")
	fs.BoolVar(&cfg.yes, "yes", false, "skip the 5 s start countdown")
	if err := cli.Parse(fs, args); err != nil {
		return cfg, err
	}

	if cfg.user == "" {
		cfg.user = os.Getenv("MQTT_USER")
	}
	if cfg.password == "" {
		cfg.password = os.Getenv("MQTT_PASSWORD")
	}
	if cfg.baseline < 8*time.Second {
		return cfg, fmt.Errorf("-baseline must be at least 8s")
	}
	switch cfg.mode {
	case "color", "brightness", "combo":
	default:
		return cfg, fmt.Errorf("unknown -mode %q", cfg.mode)
	}
	for _, t := range strings.Split(targets, ",") {
		if t = strings.TrimSpace(t); t != "" {
			cfg.targets = append(cfg.targets, t)
		}
	}
	if len(cfg.targets) == 0 {
		return cfg, fmt.Errorf("-targets is required, e.g. -targets lamp-1,lamp-2,lamp-3")
	}
	if cfg.canary == "" {
		return cfg, fmt.Errorf("-canary is required: a mains-powered device far from the targets")
	}
	for _, t := range cfg.targets {
		if t == cfg.canary {
			return cfg, fmt.Errorf("-canary must not be one of the -targets")
		}
	}
	for _, r := range strings.Split(rates, ",") {
		v, err := strconv.ParseFloat(strings.TrimSpace(r), 64)
		if err != nil || v <= 0 || v > maxRate {
			return cfg, fmt.Errorf("bad rate %q", r)
		}
		cfg.rates = append(cfg.rates, v)
	}
	return cfg, nil
}
