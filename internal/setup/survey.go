package setup

import (
	"slices"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

type refusal struct{ name, why string }

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

func model(d z2m.Device) string {
	if d.Definition == nil {
		return ""
	}
	return d.Definition.Vendor + " " + d.Definition.Model
}

func actuator(d z2m.Device) bool {
	if d.Definition == nil {
		return false
	}
	return slices.ContainsFunc(d.Definition.Exposes, func(e z2m.Expose) bool { return e.Type == "light" || e.Type == "switch" })
}
