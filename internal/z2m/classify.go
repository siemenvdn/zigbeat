package z2m

import (
	"fmt"
	"regexp"
	"slices"
)

// Role is what zigbeat does with a light.
type Role string

// The roles.
const (
	Color   Role = "color"
	Breathe Role = "breathe"
)

const accessSet = 2

const tuyaDatapoint = "TS0601"

var (
	neverDriven = []string{"switch", "cover", "fan", "lock", "climate"}

	metering = []string{"power", "energy", "current", "voltage", "power_factor"}

	inWall = []string{"switch_type", "external_switch_type", "light_type", "dimmer_mode", "operation_mode", "overload_protection"}

	notABulb = regexp.MustCompile(`(?i)\bdimm(?:er|ing)\b|plug|socket|outlet|relay|module|actuator|insert|in-wall|switch|micro|ballast|triac|phase|edge|power supply|transformer`)
)

// Classify says whether zigbeat may drive d, and how.
func Classify(d Device) (role Role, why string) {
	def := d.Definition
	switch {
	case d.Type == "Coordinator":
		return "", "the coordinator"
	case d.Type != "Router":
		return "", "battery or end device"
	case d.Disabled:
		return "", "disabled in Zigbee2MQTT"
	case !interviewed(d):
		return "", "interview not finished"
	case def == nil || !d.Supported:
		return "", "not supported by Zigbee2MQTT"
	case def.Source != "" && def.Source != "native":
		return "", def.Source + " definition"
	}
	for _, e := range def.Exposes {
		if slices.Contains(neverDriven, e.Type) {
			return "", e.Type + ", never driven"
		}
	}
	var lights []Expose
	for _, e := range def.Exposes {
		if e.Type == "light" {
			lights = append(lights, e)
		}
	}
	switch {
	case len(lights) == 0:
		return "", "no light"
	case len(lights) > 1:
		return "", "several lights"
	case lights[0].Endpoint != "":
		return "", fmt.Sprintf("several channels (endpoint %s)", lights[0].Endpoint)
	case !settable(lights[0], "state") || !settable(lights[0], "brightness"):
		return "", "on/off only"
	}
	if n := named(def.Exposes, metering); n != "" {
		return "", fmt.Sprintf("measures power (%s): a plug or module", n)
	}
	if n := named(def.Exposes, inWall); n != "" {
		return "", fmt.Sprintf("in-wall settings (%s): a module", n)
	}
	if d.ModelID == tuyaDatapoint {
		return "", "Tuya datapoint light: transitions and brightness_move are unreliable"
	}
	if notABulb.MatchString(def.Description + " " + def.Model) {
		return "", fmt.Sprintf("looks like a plug, dimmer or module: %q", def.Description)
	}
	if feature(lights[0], "color_xy") != nil {
		return Color, ""
	}
	return Breathe, ""
}

// Check says why the light called name may not be driven in role, or returns nil.
func Check(devs []Device, name string, role Role) error {
	d, ok := Find(devs, name)
	if !ok {
		return fmt.Errorf("light %q is not a Zigbee2MQTT device", name)
	}
	got, why := Classify(d)
	switch {
	case got == "":
		return fmt.Errorf("light %q is not offered by zigbeat setup (%s)", name, why)
	case role == Color && got != Color:
		return fmt.Errorf("light %q has no xy colour", name)
	}
	return nil
}

// CanaryCandidate says whether d can watch the network.
func CanaryCandidate(d Device) bool {
	return d.Type == "Router" && !d.Disabled && interviewed(d)
}

func interviewed(d Device) bool {
	return d.Interview == "SUCCESSFUL" || d.Interview == "" && d.Interviewed
}

func feature(e Expose, name string) *Expose {
	for i := range e.Features {
		if e.Features[i].Name == name {
			return &e.Features[i]
		}
	}
	return nil
}

func settable(e Expose, name string) bool {
	f := feature(e, name)
	return f != nil && f.Access&accessSet != 0
}

func named(es []Expose, names []string) string {
	for _, e := range es {
		if slices.Contains(names, e.Name) {
			return e.Name
		}
	}
	return ""
}
