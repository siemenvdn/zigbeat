// Package setup is zigbeat setup.
package setup

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

type light struct {
	name  string
	role  z2m.Role
	model string
}

func indexOf(ls []light, name string) int {
	return slices.IndexFunc(ls, func(l light) bool { return l.name == name })
}

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
			case f.Role == z2m.Color && l.role != z2m.Color:
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

const firstCopy = 2

func group(ls []light, taken []config.Fixture) []config.Fixture {
	var out []config.Fixture
	index := map[string]int{}
	for _, l := range ls {
		key := string(l.role) + "/" + stem(l.name)
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

func fixtureOf(fixtures []config.Fixture) map[string]string {
	m := map[string]string{}
	for _, f := range fixtures {
		for _, l := range f.Lights {
			m[l] = f.Name
		}
	}
	return m
}
