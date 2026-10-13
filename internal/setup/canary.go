package setup

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/lights"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

const (
	canaryReads = 3
	minAnswers  = 2
	readGap     = time.Second
)

func (s *session) chooseCanary(ctx context.Context, devs []z2m.Device, chosen []light, current string) (canary string, code int) {
	cands := canaryCandidates(devs, chosen)
	switch {
	case s.o.canary == noCanary:
		return "", 0
	case s.o.canary != "" && !slices.ContainsFunc(cands, func(d z2m.Device) bool { return d.Name == s.o.canary }):
		fmt.Fprintf(s.err, "-canary: %s is not a router outside the show\n", s.o.canary)
		return "", cli.ExitUsage
	case s.o.canary != "":
		if !s.answers(ctx, s.o.canary) {
			if ctx.Err() != nil {
				return "", s.nothing()
			}
			return "", 1
		}
		return s.o.canary, 0
	case len(cands) == 0:
		fmt.Fprintln(s.out, "no router outside the show can be the canary; the show runs without one")
		return "", 0
	}
	return s.askCanary(ctx, cands, current)
}

func canaryCandidates(devs []z2m.Device, chosen []light) []z2m.Device {
	var out []z2m.Device
	for _, d := range devs {
		if z2m.CanaryCandidate(d) && indexOf(chosen, d.Name) < 0 {
			out = append(out, d)
		}
	}
	return out
}

func (s *session) askCanary(ctx context.Context, cands []z2m.Device, current string) (canary string, code int) {
	fmt.Fprintln(s.out, "canary candidates:")
	tw := tabwriter.NewWriter(s.out, 0, 0, columnGap, ' ', 0)
	for i, d := range cands {
		fmt.Fprintf(tw, "  %d\t%s\t%s\n", i+1, d.Name, model(d))
	}
	tw.Flush()
	q := "canary, a mains device far from these lights (number, 0 for none):"
	keep := slices.ContainsFunc(cands, func(d z2m.Device) bool { return d.Name == current })
	if keep {
		q = fmt.Sprintf("canary, a mains device far from these lights (number, Enter keeps %s, 0 for none):", current)
	}
	for {
		answer, ok := s.in.ask(ctx, s.out, q)
		if !ok {
			return "", s.nothing()
		}
		name := current
		if answer != "" || !keep {
			n, err := strconv.Atoi(answer)
			if err != nil || n < 0 || n > len(cands) {
				fmt.Fprintln(s.out, "answer with a number from the list, or 0 for none")
				continue
			}
			if n == 0 {
				return "", 0
			}
			name = cands[n-1].Name
		}
		if s.answers(ctx, name) {
			return name, 0
		}
		if ctx.Err() != nil {
			return "", s.nothing()
		}
	}
}

func (s *session) answers(ctx context.Context, name string) bool {
	got := lights.ReadTimes(ctx, s.br, name, canaryReads, s.gap)
	if ctx.Err() != nil {
		return false
	}
	if len(got) < minAnswers {
		fmt.Fprintf(s.out, "%s answered %d of %d; choose another canary\n", name, len(got), canaryReads)
		return false
	}
	fmt.Fprintf(s.out, "%s answered %d of %d, slowest %d ms\n", name, len(got), canaryReads, slices.Max(got).Milliseconds())
	return true
}
