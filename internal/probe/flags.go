package probe

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/mqtt"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Flag defaults.
const (
	defaultRates    = "1,2,3,4,6,8,10,12" // fine steps where real networks knee
	defaultStep     = 20 * time.Second    // enough commands per step for a steady p95
	defaultCooldown = 10 * time.Second    // the queues of the last step empty
	defaultBaseline = 20 * time.Second    // about 20 canary reads at rest
	defaultTimeout  = 5 * time.Second     // far above a healthy answer of tens of ms
	// minBaseline gives about 8 canary reads: z2m.MinCanaryReads with a few lost.
	minBaseline = 8 * time.Second
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
	fs.StringVar(&cfg.broker, "broker", mqtt.DefaultBroker, "MQTT broker host:port")
	fs.StringVar(&cfg.user, "user", "", "MQTT username (falls back to $MQTT_USER)")
	fs.StringVar(&cfg.password, "password", "", "MQTT password (falls back to $MQTT_PASSWORD)")
	fs.StringVar(&cfg.base, "base", z2m.DefaultBaseTopic, "Zigbee2MQTT base topic")
	fs.StringVar(&targets, "targets", "", "comma-separated Zigbee2MQTT friendly names of colour lights to drive (required)")
	fs.StringVar(&cfg.canary, "canary", "", "friendly name of a mains-powered device far from the targets, used to watch the rest of the network (required)")
	fs.StringVar(&rates, "rates", defaultRates, fmt.Sprintf("comma-separated starting rates (cmd/s, all targets together); after these the probe climbs by %g%% per step until trouble", climbPercent()))
	fs.DurationVar(&cfg.step, "step", defaultStep, "duration of each rate step")
	fs.DurationVar(&cfg.cooldown, "cooldown", defaultCooldown, "pause between steps")
	fs.DurationVar(&cfg.baseline, "baseline", defaultBaseline, "canary-only measurement before the steps")
	fs.DurationVar(&cfg.timeout, "timeout", defaultTimeout, "a command or probe without answer after this counts as lost")
	fs.StringVar(&cfg.mode, "mode", "color", "command type: color, brightness or combo")
	fs.StringVar(&cfg.out, "out", "zigbeat-probe-"+time.Now().Format("20060102-150405")+".csv", "CSV file for raw samples")
	fs.BoolVar(&cfg.yes, "yes", false, "skip the "+cli.Seconds(countdown)+" start countdown")
	if err := cli.Parse(fs, args); err != nil {
		return cfg, err
	}

	if cfg.user == "" {
		cfg.user = os.Getenv("MQTT_USER")
	}
	if cfg.password == "" {
		cfg.password = os.Getenv("MQTT_PASSWORD")
	}
	if cfg.baseline < minBaseline {
		return cfg, fmt.Errorf("-baseline must be at least %v", minBaseline)
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
