// Package setup is zigbeat setup: it finds the lights in Zigbee2MQTT, proposes
// roles and fixtures, and writes the configuration.
package setup

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/siemenvdn/zigbeat/internal/config"
)

// light is a light Zigbee2MQTT offers zigbeat.
type light struct {
	name  string
	role  string // color or breathe, from z2m.Classify
	model string // vendor and model, for the table
}

// indexOf is the position of the light called name in ls, or -1.
func indexOf(ls []light, name string) int {
	return slices.IndexFunc(ls, func(l light) bool { return l.name == name })
}

// stem is a name without its trailing number: pendant-1 gives pendant. A
// device that was not renamed, called after its address, is its own stem.
func stem(name string) string {
	if hex, ok := strings.CutPrefix(name, "0x"); ok {
		if _, err := strconv.ParseUint(hex, 16, 64); err == nil {
			return name
		}
	}
	s := strings.TrimRight(strings.TrimRight(name, "0123456789"), "-_. ")
	if s == "" {
		return name
	}
	return s
}

// arrange puts the chosen lights into fixtures. Existing fixtures keep their
// name, role, mode and order, with the lights that stay. A new light joins the
// first existing fixture of its role that holds a light with the same stem;
// the others are grouped by stem and role. A light in a color fixture that has
// no xy colour any more moves out, with a note.
func arrange(existing []config.Fixture, chosen []light) (fixtures []config.Fixture, notes []string) {
	byName := map[string]light{}
	for _, l := range chosen {
		byName[l.name] = l
	}
	placed := map[string]bool{}
	for _, f := range existing {
		var keep []string
		for _, n := range f.Lights {
			l, ok := byName[n]
			switch {
			case !ok:
			case f.Role == "color" && l.role != "color":
				notes = append(notes, fmt.Sprintf("%s: no xy colour; now breathe", n))
			default:
				keep = append(keep, n)
				placed[n] = true
			}
		}
		if len(keep) > 0 {
			f.Lights = keep
			fixtures = append(fixtures, f)
		}
	}
	var fresh []light
	for _, l := range chosen {
		if !placed[l.name] && !joinExisting(fixtures, l) {
			fresh = append(fresh, l)
		}
	}
	return append(fixtures, group(fresh, fixtures)...), notes
}

// joinExisting adds l to the first fixture of its role with a light of the same stem.
func joinExisting(fixtures []config.Fixture, l light) bool {
	for i, f := range fixtures {
		if f.Role != l.role {
			continue
		}
		for _, n := range f.Lights {
			if stem(n) == stem(l.name) {
				fixtures[i].Lights = append(fixtures[i].Lights, l.name)
				return true
			}
		}
	}
	return false
}

// firstCopy is the suffix of the first fixture that would take a name already
// taken: the original counts as the first.
const firstCopy = 2

// group makes new fixtures of lights with the same stem and role, in order:
// named after the stem, or after the light when it is alone; a name that is
// taken gets -2, -3 and so on.
func group(ls []light, taken []config.Fixture) []config.Fixture {
	var out []config.Fixture
	index := map[string]int{}
	for _, l := range ls {
		key := l.role + "/" + stem(l.name)
		if i, ok := index[key]; ok {
			out[i].Lights = append(out[i].Lights, l.name)
			continue
		}
		index[key] = len(out)
		out = append(out, config.Fixture{Role: l.role, Lights: []string{l.name}})
	}
	names := map[string]bool{}
	for _, f := range taken {
		names[f.Name] = true
	}
	for i, f := range out {
		name := f.Lights[0]
		if len(f.Lights) > 1 {
			name = stem(name)
		}
		unique := name
		for n := firstCopy; names[unique]; n++ {
			unique = fmt.Sprintf("%s-%d", name, n)
		}
		names[unique] = true
		out[i].Name = unique
	}
	return out
}

// fixtureOf maps every light to the name of its fixture.
func fixtureOf(fixtures []config.Fixture) map[string]string {
	m := map[string]string{}
	for _, f := range fixtures {
		for _, l := range f.Lights {
			m[l] = f.Name
		}
	}
	return m
}

// lightsOf lists the lights of fixtures in order.
func lightsOf(fixtures []config.Fixture) []string {
	var out []string
	for _, f := range fixtures {
		out = append(out, f.Lights...)
	}
	return out
}
