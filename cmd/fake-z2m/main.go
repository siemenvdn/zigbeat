// fake-z2m simulates Zigbee2MQTT with a single coordinator queue of limited
// throughput, so zigbee-stress can be tested without real lights.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/siemenvdn/zigbeat/internal/mqtt"
)

type job struct {
	device string
	kind   string
	body   map[string]any
}

func main() {
	broker := flag.String("broker", "127.0.0.1:1883", "MQTT broker")
	base := flag.String("base", "zigbee2mqtt", "base topic")
	devices := flag.String("devices", "lamp-1,lamp-2,lamp-3,far-lamp", "simulated devices")
	service := flag.Duration("service", 90*time.Millisecond, "air time per Zigbee command")
	jitter := flag.Duration("jitter", 20*time.Millisecond, "random extra delay per command")
	queue := flag.Int("queue", 64, "coordinator queue depth before commands fail")
	flag.Parse()

	c, err := mqtt.Dial(*broker, mqtt.Options{ClientID: "fake-z2m", Username: os.Getenv("MQTT_USER"), Password: os.Getenv("MQTT_PASSWORD")})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var mu sync.Mutex
	state := map[string]map[string]any{}
	known := map[string]bool{}
	for _, d := range strings.Split(*devices, ",") {
		known[d] = true
		state[d] = map[string]any{"state": "ON", "brightness": 180, "color_mode": "xy", "color": map[string]any{"x": 0.47, "y": 0.39}, "linkquality": 120}
	}

	jobs := make(chan job, *queue)
	publishLog := func(level, msg string) {
		b, _ := json.Marshal(map[string]string{"level": level, "message": msg, "namespace": "z2m"})
		_ = c.Publish(*base+"/bridge/logging", b)
	}
	go func() {
		for j := range jobs {
			time.Sleep(*service + time.Duration(rand.Int63n(int64(*jitter)+1)))
			mu.Lock()
			st := state[j.device]
			if j.kind == "set" {
				for k, v := range j.body {
					if k == "transition" {
						continue
					}
					st[k] = v
					if k == "color" {
						st["color_mode"] = "xy"
					}
					if k == "color_temp" {
						st["color_mode"] = "color_temp"
					}
				}
			}
			b, _ := json.Marshal(st)
			mu.Unlock()
			_ = c.Publish(*base+"/"+j.device, b)
		}
	}()

	handle := func(kind string) mqtt.Handler {
		return func(topic string, payload []byte, _ time.Time) {
			parts := strings.Split(topic, "/")
			if len(parts) != 3 || !known[parts[1]] {
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
	fmt.Printf("fake-z2m ready: capacity about %.1f cmd/s\n", float64(time.Second)/float64(*service+*jitter/2))

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	select {
	case <-sig:
	case <-c.Done():
		fmt.Fprintln(os.Stderr, c.Err())
	}
}
