// fake-z2m simulates Zigbee2MQTT with a single coordinator queue of limited
// throughput, so zigbeat can be tested without real lights.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/mqtt"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Flag defaults: a network a bit busier than a real one, so zigbeat's limits
// and backoff get exercised.
const (
	defaultService = 90 * time.Millisecond // about 10 frames/s
	defaultJitter  = 20 * time.Millisecond // spreads the latencies, like a mesh
	defaultQueue   = 64                    // deep enough that only an overload fills it
)

const topicParts = 3 // base/device/set or base/device/get

func main() {
	broker := flag.String("broker", mqtt.DefaultBroker, "MQTT broker")
	base := flag.String("base", z2m.DefaultBaseTopic, "base topic")
	devices := flag.String("devices", defaultDevices, "simulated devices, each with an optional :kind (color, ct, white, hs, relay, plug, dimmer, tuya or sensor; default color)")
	service := flag.Duration("service", defaultService, "air time per Zigbee frame")
	jitter := flag.Duration("jitter", defaultJitter, "random extra delay per command")
	queue := flag.Int("queue", defaultQueue, "coordinator queue depth before commands fail")
	loss := flag.Float64("loss", 0, "share of commands that never get an answer, 0 to 1")
	groups := flag.String("groups", "", "simulated Zigbee2MQTT groups, published retained on bridge/groups")
	logLand := flag.Bool("log", false, "print every set command when the simulated light applies it")
	retainDevices := flag.Bool("bridge-devices", true, "publish bridge/devices retained; false clears it, like Zigbee2MQTT without retain")
	flag.Parse()
	names, kindOf, err := parseDevices(*devices)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitUsage)
	}

	c, err := mqtt.Dial(*broker, mqtt.Options{ClientID: "fake-z2m", Username: os.Getenv("MQTT_USER"), Password: os.Getenv("MQTT_PASSWORD")})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	state := map[string]map[string]any{}
	known := map[string]bool{}
	for _, d := range names {
		known[d] = true
		state[d] = initialState(kindOf[d])
	}
	var devs []byte // empty clears the retained message
	if *retainDevices {
		devs = bridgeDevices(names, kindOf)
	}
	_ = c.PublishRetained(*base+"/bridge/devices", devs)
	gs := []map[string]any{}
	for i, g := range strings.Split(*groups, ",") {
		if g != "" {
			gs = append(gs, map[string]any{"id": i + 1, "friendly_name": g})
		}
	}
	b, _ := json.Marshal(gs)
	_ = c.PublishRetained(*base+"/bridge/groups", b)

	jobs := make(chan job, *queue)
	publishLog := func(level, msg string) {
		b, _ := json.Marshal(map[string]string{"level": level, "message": msg, "namespace": "z2m"})
		_ = c.Publish(*base+"/bridge/logging", b)
	}
	co := &coordinator{c: c, base: *base, service: *service, jitter: *jitter, loss: *loss, logLand: *logLand, state: state}
	go co.run(jobs)

	handle := func(kind string) mqtt.Handler {
		return func(topic string, payload []byte, _ time.Time) {
			parts := strings.Split(topic, "/")
			if len(parts) != topicParts || !known[parts[1]] {
				return
			}
			var body map[string]any
			if json.Unmarshal(payload, &body) != nil {
				return
			}
			select {
			case jobs <- job{device: parts[1], kind: kind, body: body}:
			default:
				publishLog("error", fmt.Sprintf("Publish '%s' to '%s' failed: queue full", kind, parts[1]))
			}
		}
	}
	if err := c.Subscribe(*base+"/+/set", handle("set")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := c.Subscribe(*base+"/+/get", handle("get")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("fake-z2m ready: capacity about %.1f frames/s\n", float64(time.Second)/float64(*service+*jitter/2))

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	select {
	case <-sig:
	case <-c.Done():
		fmt.Fprintln(os.Stderr, c.Err())
	}
}
