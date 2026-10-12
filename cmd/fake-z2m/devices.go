package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// defaultDevices: three colour lights and a white one for the show, a colour
// light as canary, and devices zigbeat must never drive.
const defaultDevices = "lamp-1,lamp-2,lamp-3,lamp-4:white,far-lamp,plug-1:plug,relay-1:relay,dimmer-1:dimmer,sensor-1:sensor"

// Access bits of an expose, as in herdsman-converters: state, set, get.
const (
	accessState = 1
	accessAll   = 7
)

// kind is a device as Zigbee2MQTT describes it in bridge/devices. The kinds are
// written from Zigbee2MQTT's test fixtures and herdsman-converters' device
// definitions, not from zigbeat's parser.
type kind struct {
	typ, power, modelID, model, vendor, description string
	exposes                                         []any
	color                                           bool // the state has a colour
}

const mains, battery = "Mains (single phase)", "Battery"

var kinds = map[string]kind{
	"color":  {"Router", mains, "LLC020", "7146060PH", "Philips", "Hue Go", []any{light(state, brightness, colorTemp, colorXY, colorHS)}, true},
	"ct":     {"Router", mains, "TRADFRI bulb E27 WS opal 980lm", "LED1545G12", "IKEA", "TRADFRI bulb E26/E27, white spectrum, globe, opal, 980 lm", []any{light(state, brightness, colorTemp)}, false},
	"white":  {"Router", mains, "TRADFRI bulb E27 W opal 1000lm", "LED1623G12", "IKEA", "TRADFRI bulb E27, white, globe, opal, 1000 lm", []any{light(state, brightness)}, false},
	"hs":     {"Router", mains, "Classic A60 RGBW", "AA69697", "OSRAM", "Classic A60 RGBW", []any{light(state, brightness, colorTemp, colorHS)}, true},
	"relay":  {"Router", mains, "01MINIZB", "ZBMINI", "SONOFF", "Zigbee two way smart switch", []any{onOff()}, false},
	"plug":   {"Router", mains, "lumi.plug", "ZNCZ02LM", "Xiaomi", "Mi smart plug", []any{onOff(), numeric("power", accessState), numeric("energy", accessState)}, false},
	"dimmer": {"Router", mains, "TS110E", "TS110E_1gang_1", "Tuya", "1 channel dimmer", []any{light(state, brightness), enum("switch_type")}, false},
	"tuya":   {"Router", mains, "TS0601", "TS0601_light", "Tuya", "Smart light", []any{light(state, brightness, colorXY)}, true},
	"sensor": {"EndDevice", battery, "lumi.weather", "WSDCGQ11LM", "Aqara", "Temperature and humidity sensor", []any{numeric("temperature", accessState), numeric("battery", accessState)}, false},
}

var (
	state      = feature("binary", "state")
	brightness = feature("numeric", "brightness")
	colorTemp  = feature("numeric", "color_temp")
	colorXY    = composite("color_xy", feature("numeric", "x"), feature("numeric", "y"))
	colorHS    = composite("color_hs", feature("numeric", "hue"), feature("numeric", "saturation"))
)

func feature(typ, name string) map[string]any {
	return map[string]any{"type": typ, "name": name, "property": name, "access": accessAll}
}

func composite(name string, fs ...map[string]any) map[string]any {
	return map[string]any{"type": "composite", "name": name, "property": "color", "access": accessAll, "features": fs}
}

func light(fs ...map[string]any) map[string]any {
	return map[string]any{"type": "light", "features": fs}
}

func onOff() map[string]any {
	return map[string]any{"type": "switch", "features": []map[string]any{state}}
}

func numeric(name string, access int) map[string]any {
	return map[string]any{"type": "numeric", "name": name, "property": name, "access": access}
}

func enum(name string) map[string]any { return feature("enum", name) }

// parseDevices reads -devices: names, each with an optional :kind.
func parseDevices(list string) (names []string, kindOf map[string]string, err error) {
	kindOf = map[string]string{}
	for _, d := range strings.Split(list, ",") {
		name, k, _ := strings.Cut(d, ":")
		if k == "" {
			k = "color"
		}
		if _, ok := kinds[k]; !ok || name == "" {
			return nil, nil, fmt.Errorf("-devices: bad entry %q; kinds are color, ct, white, hs, relay, plug, dimmer, tuya and sensor", d)
		}
		names = append(names, name)
		kindOf[name] = k
	}
	return names, kindOf, nil
}

// bridgeDevices is the bridge/devices message: the coordinator, then every device.
func bridgeDevices(names []string, kindOf map[string]string) []byte {
	ds := []map[string]any{{
		"ieee_address": "0x00124b00120144ae", "friendly_name": "Coordinator", "type": "Coordinator",
		"disabled": false, "supported": true, "interview_state": "SUCCESSFUL", "interview_completed": true,
	}}
	for i, n := range names {
		k := kinds[kindOf[n]]
		ds = append(ds, map[string]any{
			"ieee_address": fmt.Sprintf("0x%016x", i+1), "friendly_name": n, "type": k.typ,
			"disabled": false, "supported": true, "interview_state": "SUCCESSFUL", "interview_completed": true,
			"model_id": k.modelID, "power_source": k.power,
			"definition": map[string]any{"source": "native", "model": k.model, "vendor": k.vendor, "description": k.description, "exposes": k.exposes},
		})
	}
	b, _ := json.Marshal(ds)
	return b
}

// initialState is a device before the show: on at about 70 %, in a warm white
// when it has colour, like a light that was on.
func initialState(k string) map[string]any {
	st := map[string]any{"state": "ON", "brightness": 180, "linkquality": 120}
	if kinds[k].color {
		st["color_mode"] = "xy"
		st["color"] = map[string]any{"x": 0.47, "y": 0.39}
	}
	return st
}
