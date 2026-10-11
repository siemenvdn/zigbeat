// Package probe measures how many light commands per second a Zigbee network
// handles before commands lag or the rest of the network slows down.
package probe

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/siemenvdn/zigbeat/internal/mqtt"
	"github.com/siemenvdn/zigbeat/internal/z2m"
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

// errUsage means the flag package already printed the problem and the usage.
var errUsage = errors.New("usage")

type stateAt struct {
	s  z2m.State
	at time.Time
}

// Run executes `zigbeat probe` with the given arguments and returns an exit code.
func Run(args []string) int {
	cfg, err := parseFlags(args)
	if err == flag.ErrHelp {
		return 0
	}
	if err == errUsage {
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	client, err := mqtt.Dial(cfg.broker, mqtt.Options{
		ClientID:  fmt.Sprintf("zigbeat-probe-%d", os.Getpid()),
		Username:  cfg.user,
		Password:  cfg.password,
		KeepAlive: 20 * time.Second,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "mqtt: %v\n", err)
		return 1
	}
	defer client.Close()
	bridge := z2m.New(client, cfg.base)

	rec := &recorder{}
	ph := &phaseState{name: "setup"}
	gen := newGenerator(cfg.mode)
	tracker := newCmdTracker(rec, cfg.timeout, keyFunc(cfg.mode))
	probe := &canaryProbe{device: cfg.canary, rec: rec, timeout: cfg.timeout}

	var lmu sync.Mutex
	last := map[string]stateAt{}
	for _, t := range cfg.targets {
		t := t
		err := bridge.OnState(t, func(s z2m.State, at time.Time) {
			lmu.Lock()
			last[t] = stateAt{s, at}
			lmu.Unlock()
			tracker.onState(t, s, at)
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "subscribe: %v\n", err)
			return 1
		}
	}
	canaryAnswered := make(chan struct{}, 1)
	if err := bridge.OnState(cfg.canary, func(_ z2m.State, at time.Time) {
		probe.onState(at)
		select {
		case canaryAnswered <- struct{}{}:
		default:
		}
	}); err != nil {
		fmt.Fprintf(os.Stderr, "subscribe: %v\n", err)
		return 1
	}
	if err := bridge.OnLog(func(e z2m.LogEntry, at time.Time) {
		if e.Level == "error" || e.Level == "warning" {
			rec.addLog(logEvent{at: at, level: e.Level, msg: e.Message})
		}
	}); err != nil {
		fmt.Fprintf(os.Stderr, "subscribe: %v\n", err)
		return 1
	}

	answeredSince := func(dev string, since time.Time) (z2m.State, bool) {
		lmu.Lock()
		defer lmu.Unlock()
		v, ok := last[dev]
		return v.s, ok && v.at.After(since)
	}

	// Snapshot the current light state so it can be put back afterwards.
	snapAt := time.Now()
	for _, t := range cfg.targets {
		_ = bridge.Get(t, map[string]string{"state": "", "brightness": "", "color": "", "color_temp": ""})
	}
	snaps := map[string]z2m.State{}
	if !waitFor(5*time.Second, func() bool {
		for _, t := range cfg.targets {
			s, ok := answeredSince(t, snapAt)
			if !ok {
				return false
			}
			snaps[t] = s
		}
		return true
	}) {
		for _, t := range cfg.targets {
			if _, ok := snaps[t]; !ok {
				fmt.Fprintf(os.Stderr, "no answer from %s; check the friendly name and that it is powered\n", t)
			}
		}
		return 1
	}

	_ = bridge.Get(cfg.canary, map[string]string{"state": ""})
	select {
	case <-canaryAnswered:
	case <-time.After(5 * time.Second):
		fmt.Fprintf(os.Stderr, "canary %s does not answer; pick another device with -canary\n", cfg.canary)
		return 1
	}

	snapJSON, _ := json.Marshal(snaps)
	est := cfg.baseline + time.Duration(len(cfg.rates))*(cfg.step+cfg.cooldown+2*time.Second)
	fmt.Printf("zigbeat probe: %d targets, canary %s, mode %s, palette %d\n", len(cfg.targets), cfg.canary, cfg.mode, gen.paletteSize())
	fmt.Printf("rates %v cmd/s, %s per step, %s cooldown, about %s in total\n", cfg.rates, cfg.step, cfg.cooldown, est.Round(10*time.Second))
	fmt.Printf("current state (for manual restore): %s\n", snapJSON)
	if !cfg.yes {
		fmt.Println("The lights will flash. Starting in 5 s, Ctrl-C to cancel.")
		if !sleepCtx(ctx, client, 5*time.Second) {
			fmt.Println("cancelled, nothing was changed")
			return 130
		}
	}

	bg, stopBg := context.WithCancel(context.Background())
	defer stopBg()
	// Irregular probe interval so probes never lock in phase with the command rate.
	go func() {
		for {
			d := 700*time.Millisecond + time.Duration(rand.Int63n(int64(600*time.Millisecond)))
			select {
			case <-bg.Done():
				return
			case now := <-time.After(d):
				probe.tick(now, ph, func() error { return bridge.Get(cfg.canary, map[string]string{"state": ""}) })
			}
		}
	}()
	go every(bg, 200*time.Millisecond, tracker.expire)

	restore := func() {
		ph.set("restore", 0)
		fmt.Println("\nrestoring lights")
		failed := 0
		for _, t := range cfg.targets {
			ok := false
			for attempt := 0; attempt < 2 && !ok; attempt++ {
				sent := time.Now()
				_ = bridge.Set(t, restorePayload(snaps[t]))
				ok = waitFor(3*time.Second, func() bool { _, ok := answeredSince(t, sent); return ok })
			}
			if !ok {
				failed++
				fmt.Printf("  could not confirm %s, set it back by hand\n", t)
			}
		}
		if failed == 0 {
			fmt.Println("  all targets back to their previous state")
		}
	}

	finish := func(results []stepResult, base latencyStats) {
		stopBg()
		if len(results) > 0 {
			printSummary(os.Stdout, results, base, cfg.canary)
		}
		samples, logs := rec.snapshot()
		if err := writeCSV(cfg.out, samples, logs); err != nil {
			fmt.Fprintf(os.Stderr, "csv: %v\n", err)
		} else {
			fmt.Printf("raw samples: %s\n", cfg.out)
		}
	}

	var intOnce sync.Once
	interrupted := func() bool {
		if ctx.Err() == nil {
			return false
		}
		intOnce.Do(func() {
			stopSignals()
			fmt.Println("\ninterrupted (Ctrl-C again to quit without restoring)")
		})
		return true
	}
	var lostOnce sync.Once
	connLost := func() bool {
		select {
		case <-client.Done():
			lostOnce.Do(func() {
				fmt.Fprintf(os.Stderr, "\nmqtt connection lost: %v\nrestore by hand from: %s\n", client.Err(), snapJSON)
			})
			return true
		default:
			return false
		}
	}

	// Baseline: canary only, nothing else on the air from us.
	ph.set("baseline", 0)
	fmt.Printf("measuring baseline for %s\n", cfg.baseline)
	if !sleepCtx(ctx, client, cfg.baseline) {
		if connLost() {
			return 1
		}
		interrupted()
		finish(nil, latencyStats{})
		return 130
	}
	samples, _ := rec.snapshot()
	var baseSamples []sample
	for _, s := range samples {
		if s.kind == "canary" && s.phase == "baseline" {
			baseSamples = append(baseSamples, s)
		}
	}
	base := statsOf(baseSamples)
	if base.n < 3 {
		fmt.Fprintln(os.Stderr, "canary answered too rarely during the baseline, aborting")
		finish(nil, base)
		return 1
	}
	lim := limits{canaryBaseP50: base.p50, canaryBaseP95: base.p95}
	fmt.Printf("canary at rest: p50 %s ms, p95 %s ms (step fails above %s ms)\n", ms(base.p50), ms(base.p95), ms(lim.canaryLimit()))

	ph.set("prep", 0)
	for _, t := range cfg.targets {
		_ = bridge.Set(t, map[string]any{"state": "ON", "brightness": 200, "transition": 0})
	}
	sleepCtx(ctx, client, 2*time.Second)

	// Self-check: every target must echo a command in a way we can match.
	ph.set("selfcheck", 0)
	for _, t := range cfg.targets {
		cmd := gen.nextFor(t)
		tracker.add(t, cmd.key, pending{sent: time.Now(), phase: "selfcheck"})
		_ = bridge.Set(t, cmd.payload)
	}
	waitFor(cfg.timeout+300*time.Millisecond, func() bool { return tracker.inflight() == 0 })
	tracker.expire(time.Now().Add(cfg.timeout))
	samples, _ = rec.snapshot()
	matched := 0
	for _, s := range samples {
		if s.phase == "selfcheck" && s.ok {
			matched++
		}
	}
	if matched < len(cfg.targets) {
		fmt.Fprintf(os.Stderr, "self-check failed: only %d of %d targets echoed a recognisable state\n", matched, len(cfg.targets))
		for _, t := range cfg.targets {
			lmu.Lock()
			b, _ := json.Marshal(last[t].s)
			lmu.Unlock()
			fmt.Fprintf(os.Stderr, "  last state of %s: %s\n", t, b)
		}
		fmt.Fprintln(os.Stderr, "try again with -mode brightness")
		restore()
		finish(nil, base)
		return 1
	}
	fmt.Printf("self-check ok, all %d targets answer\n", matched)

	var results []stepResult
	printHeader(os.Stdout)
	for _, rate := range cfg.rates {
		if interrupted() || connLost() {
			break
		}
		res, ok := runStep(ctx, client, bridge, cfg, rate, gen, tracker, rec, ph, lim)
		results = append(results, res)
		printRow(os.Stdout, res)
		if !ok {
			break
		}
		if res.verdict == "STOP" {
			waitRecovery(ctx, client, probe, ph, lim)
			break
		}
		ph.set("cooldown", rate)
		if !sleepCtx(ctx, client, cfg.cooldown) {
			break
		}
	}

	if connLost() {
		finish(results, base)
		return 1
	}
	wasInterrupted := interrupted()
	restore()
	finish(results, base)
	if wasInterrupted {
		return 130
	}
	return 0
}

func runStep(ctx context.Context, client *mqtt.Client, bridge *z2m.Bridge, cfg config, rate float64,
	gen *generator, tracker *cmdTracker, rec *recorder, ph *phaseState, lim limits) (stepResult, bool) {
	res := stepResult{rate: rate}
	ph.set("step", rate)
	start := time.Now()
	deadline := start.Add(cfg.step)
	interval := time.Duration(float64(time.Second) / rate)
	next := start
	completed := true
	for i := 0; next.Before(deadline); i++ {
		if !sleepCtx(ctx, client, time.Until(next)) {
			completed = false
			break
		}
		dev := cfg.targets[i%len(cfg.targets)]
		cmd := gen.nextFor(dev)
		tracker.add(dev, cmd.key, pending{sent: time.Now(), phase: "step", rate: rate})
		_ = bridge.Set(dev, cmd.payload)
		res.sent++
		next = next.Add(interval)
	}

	ph.set("drain", rate)
	waitFor(cfg.timeout+300*time.Millisecond, func() bool { return tracker.inflight() == 0 })
	tracker.expire(time.Now().Add(cfg.timeout))
	end := time.Now()

	samples, logs := rec.snapshot()
	var cmds, canary []sample
	for _, s := range samples {
		if s.rate != rate {
			continue
		}
		switch {
		case s.kind == "cmd" && s.phase == "step":
			cmds = append(cmds, s)
		case s.kind == "canary" && (s.phase == "step" || s.phase == "drain"):
			canary = append(canary, s)
		}
	}
	for _, l := range logs {
		if l.at.Before(start) || l.at.After(end) {
			continue
		}
		if l.level == "error" {
			res.errs++
		} else {
			res.warns++
		}
	}
	res.cmd = statsOf(cmds)
	res.canary = statsOf(canary)
	evaluate(&res, cmds, lim)
	if !completed && res.verdict != "STOP" {
		res.verdict = "PARTIAL"
		res.reasons = append(res.reasons, "step interrupted")
	}
	return res, completed
}

func waitRecovery(ctx context.Context, client *mqtt.Client, probe *canaryProbe, ph *phaseState, lim limits) {
	ph.set("recovery", 0)
	fmt.Println("stopping here, waiting for the network to settle")
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if !sleepCtx(ctx, client, time.Second) {
			return
		}
		recent := probe.recentOK(3)
		if len(recent) < 3 {
			continue
		}
		settled := true
		for _, d := range recent {
			if d > lim.canaryLimit() {
				settled = false
			}
		}
		if settled {
			fmt.Println("network settled")
			return
		}
	}
	fmt.Println("network still slow after 60 s; check the Zigbee2MQTT logs")
}

func restorePayload(s z2m.State) map[string]any {
	if strings.EqualFold(s.State, "OFF") {
		return map[string]any{"state": "OFF", "transition": 0}
	}
	p := map[string]any{"state": "ON", "transition": 0}
	if s.Brightness != nil {
		p["brightness"] = *s.Brightness
	}
	switch {
	case s.ColorMode == "color_temp" && s.ColorTemp != nil:
		p["color_temp"] = *s.ColorTemp
	case s.Color != nil:
		p["color"] = s.Color
	}
	return p
}

func every(ctx context.Context, d time.Duration, fn func(time.Time)) {
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case now := <-t.C:
			fn(now)
		case <-ctx.Done():
			return
		}
	}
}

func sleepCtx(ctx context.Context, client *mqtt.Client, d time.Duration) bool {
	if d <= 0 {
		select {
		case <-ctx.Done():
			return false
		case <-client.Done():
			return false
		default:
			return true
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	case <-client.Done():
		return false
	}
}

func waitFor(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for {
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func parseFlags(args []string) (config, error) {
	var cfg config
	var targets, rates string
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: zigbeat probe -targets <names> -canary <name> [flags]")
		fmt.Fprintln(fs.Output(), "\nMeasures how many commands per second your Zigbee network handles. See docs/probe.md.")
		fmt.Fprintln(fs.Output())
		fs.PrintDefaults()
	}
	fs.StringVar(&cfg.broker, "broker", "127.0.0.1:1883", "MQTT broker host:port")
	fs.StringVar(&cfg.user, "user", "", "MQTT username (falls back to $MQTT_USER)")
	fs.StringVar(&cfg.password, "password", "", "MQTT password (falls back to $MQTT_PASSWORD)")
	fs.StringVar(&cfg.base, "base", "zigbee2mqtt", "Zigbee2MQTT base topic")
	fs.StringVar(&targets, "targets", "", "comma-separated Zigbee2MQTT friendly names of colour lights to drive (required)")
	fs.StringVar(&cfg.canary, "canary", "", "friendly name of a mains-powered device far from the targets, used to watch the rest of the network (required)")
	fs.StringVar(&rates, "rates", "1,2,3,4,6,8,10,12", "comma-separated command rates (cmd/s, all targets together)")
	fs.DurationVar(&cfg.step, "step", 20*time.Second, "duration of each rate step")
	fs.DurationVar(&cfg.cooldown, "cooldown", 10*time.Second, "pause between steps")
	fs.DurationVar(&cfg.baseline, "baseline", 20*time.Second, "canary-only measurement before the steps")
	fs.DurationVar(&cfg.timeout, "timeout", 5*time.Second, "a command or probe without answer after this counts as lost")
	fs.StringVar(&cfg.mode, "mode", "color", "command type: color, brightness or combo")
	fs.StringVar(&cfg.out, "out", "zigbeat-probe-"+time.Now().Format("20060102-150405")+".csv", "CSV file for raw samples")
	fs.BoolVar(&cfg.yes, "yes", false, "skip the 5 s start countdown")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return cfg, err
		}
		return cfg, errUsage
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
		if err != nil || v <= 0 {
			return cfg, fmt.Errorf("bad rate %q", r)
		}
		cfg.rates = append(cfg.rates, v)
	}
	return cfg, nil
}
