package setup

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

const (
	canaryReads = 3           // enough to see it answers; the show measures it at every start
	minAnswers  = 2           // one lost read is noise
	readGap     = time.Second // about as often as the show reads the canary
)

// chooseCanary returns the canary: from -canary, or asked from the routers
// that are not in the show. code is not 0 when setup must stop.
func (s *session) chooseCanary(ctx context.Context, devs []z2m.Device, chosen []light, current string) (canary string, code int) {
	var cands []z2m.Device
	for _, d := range devs {
		if z2m.CanaryCandidate(d) && indexOf(chosen, d.Name) < 0 {
			cands = append(cands, d)
		}
	}
	names := make([]string, len(cands))
	for i, d := range cands {
		names[i] = d.Name
	}
	switch {
	case s.o.canary == noCanary:
		return "", 0
	case s.o.canary != "" && !slices.Contains(names, s.o.canary):
		fmt.Fprintf(s.err, "-canary: %s is not a router outside the show\n", s.o.canary)
		return "", cli.ExitUsage
	case s.o.canary != "":
		if !s.answers(ctx, s.o.canary) {
			return "", 1
		}
		return s.o.canary, 0
	case len(cands) == 0:
		fmt.Fprintln(s.out, "no router outside the show can be the canary; the show runs without one")
		return "", 0
	}
	fmt.Fprintln(s.out, "canary candidates:")
	tw := tabwriter.NewWriter(s.out, 0, 0, columnGap, ' ', 0)
	for i, d := range cands {
		fmt.Fprintf(tw, "  %d\t%s\t%s %s\n", i+1, d.Name, d.Definition.Vendor, d.Definition.Model)
	}
	tw.Flush()
	q := "canary, a mains device far from these lights (number, 0 for none):"
	keep := slices.Contains(names, current)
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
			if err != nil || n < 0 || n > len(names) {
				fmt.Fprintln(s.out, "answer with a number from the list, or 0 for none")
				continue
			}
			if n == 0 {
				return "", 0
			}
			name = names[n-1]
		}
		if s.answers(ctx, name) {
			return name, 0
		}
		if ctx.Err() != nil {
			return "", s.nothing()
		}
	}
}

// answers reads name canaryReads times, s.gap apart, and says whether it
// answered at least minAnswers times.
func (s *session) answers(ctx context.Context, name string) bool {
	var got []time.Duration
	for i := range canaryReads {
		if i > 0 {
			select {
			case <-ctx.Done():
				return false
			case <-time.After(s.gap):
			}
		}
		if d, ok := s.br.Read(ctx, name); ok {
			got = append(got, d)
		}
	}
	if len(got) < minAnswers {
		fmt.Fprintf(s.out, "%s answered %d of %d; choose another canary\n", name, len(got), canaryReads)
		return false
	}
	fmt.Fprintf(s.out, "%s answered %d of %d, slowest %d ms\n", name, len(got), canaryReads, slices.Max(got).Milliseconds())
	return true
}
