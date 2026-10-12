package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/mqtt"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

type job struct {
	device string
	kind   string
	body   map[string]any
}

// coordinator is the simulated Zigbee coordinator: one queue, air time per frame.
// Only its run goroutine touches state.
type coordinator struct {
	c               *mqtt.Client
	base            string
	service, jitter time.Duration
	loss            float64
	logLand         bool
	state           map[string]map[string]any
}

func (co *coordinator) run(jobs <-chan job) {
	for j := range jobs {
		co.apply(j)
	}
}

// apply sends j over the simulated air and publishes the new state, like Zigbee2MQTT.
func (co *coordinator) apply(j job) {
	frames := 1
	if j.kind == "set" {
		frames = max(1, z2m.Frames(j.body))
	}
	time.Sleep(time.Duration(frames) * (co.service + time.Duration(rand.Int63n(int64(co.jitter)+1))))
	if rand.Float64() < co.loss {
		return
	}
	st := co.state[j.device]
	echo := j.kind == "get"
	if j.kind == "set" {
		for k, v := range j.body {
			// Like Zigbee2MQTT: a brightness_move alone publishes no state.
			if k == "transition" || k == "brightness_move" {
				continue
			}
			echo = true
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
	if co.logLand && j.kind == "set" {
		body, _ := json.Marshal(j.body)
		fmt.Printf("land %s %s %s\n", time.Now().Format(cli.ClockMS), j.device, body)
	}
	if echo {
		_ = co.c.Publish(co.base+"/"+j.device, b)
	}
}
