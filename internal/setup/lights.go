package setup

import (
	"context"
	"fmt"
	"slices"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// chooseLights returns the lights for the show: from -lights, or asked.
func (s *session) chooseLights(ctx context.Context, devs []z2m.Device, offered []light, marked map[string]bool) ([]light, int) {
	if s.o.lights != nil {
		var out []light
		for _, n := range s.o.lights {
			i := slices.IndexFunc(offered, func(l light) bool { return l.name == n })
			if i < 0 {
				fmt.Fprintf(s.err, "-lights: %s is not offered (%s)\n", n, whyNot(devs, n))
				return nil, cli.ExitUsage
			}
			out = append(out, offered[i])
		}
		return out, 0
	}
	var keep []int
	for i, l := range offered {
		if marked[l.name] {
			keep = append(keep, i)
		}
	}
	q := "lights for the show (e.g. 1-3,7):"
	if len(keep) > 0 {
		q = fmt.Sprintf("lights for the show (e.g. 1-3,7; Enter keeps the %d marked *):", len(keep))
	}
	for {
		answer, ok := s.in.ask(ctx, s.out, q)
		if !ok {
			return nil, s.nothing()
		}
		picked := keep
		if answer != "" || len(keep) == 0 {
			var err error
			if picked, err = parseSelection(answer, len(offered)); err != nil {
				fmt.Fprintln(s.out, err)
				continue
			}
		}
		var out []light
		for _, i := range picked {
			out = append(out, offered[i])
		}
		return out, 0
	}
}

// whyNot says why zigbeat setup does not offer the light called name.
func whyNot(devs []z2m.Device, name string) string {
	d, ok := z2m.Find(devs, name)
	if !ok {
		return "not a Zigbee2MQTT device any more"
	}
	_, why := z2m.Classify(d)
	return why
}
