package setup

import (
	"slices"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// refusal is a device with a light or a switch that zigbeat may not drive.
type refusal struct{ name, why string }

// survey sorts the devices into the lights zigbeat may drive, the lights and
// switches it may not, and a count of the rest. The coordinator is left out.
func survey(devs []z2m.Device) (offered []light, refused []refusal, others int) {
	for _, d := range devs {
		if d.Type == "Coordinator" {
			continue
		}
		role, why := z2m.Classify(d)
		switch {
		case role != "":
			offered = append(offered, light{name: d.Name, role: role, model: model(d)})
		case actuator(d):
			refused = append(refused, refusal{d.Name, why})
		default:
			others++
		}
	}
	return offered, refused, others
}

// model is the vendor and model of d; empty for a device Zigbee2MQTT does not know.
func model(d z2m.Device) string {
	if d.Definition == nil {
		return ""
	}
	return d.Definition.Vendor + " " + d.Definition.Model
}

// actuator says whether d has a light or a switch: worth saying why it is not offered.
func actuator(d z2m.Device) bool {
	if d.Definition == nil {
		return false
	}
	return slices.ContainsFunc(d.Definition.Exposes, func(e z2m.Expose) bool { return e.Type == "light" || e.Type == "switch" })
}
