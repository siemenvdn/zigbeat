package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/lights"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Run executes zigbeat setup and returns an exit code.
func Run(args []string) int {
	o, err := parseFlags(args)
	if code, stop := cli.Exit(err); stop {
		return code
	}
	if err := o.load(); err != nil {
		return cli.Fail(err)
	}
	ctx, stop := cli.StopContext()
	defer stop()
	br, err := lights.Dial(o.mqtt, "setup", lights.Watch{})
	if err != nil {
		return cli.Fail(err)
	}
	defer br.Close()
	s := &session{o: o, br: br, in: newInput(os.Stdin), out: os.Stdout, err: os.Stderr, today: time.Now(), gap: readGap}
	return s.run(ctx)
}

// session is one run of zigbeat setup.
type session struct {
	o        options
	br       zigbee
	in       *input
	out, err io.Writer
	today    time.Time
	gap      time.Duration // between canary reads; tests use 0
}

func (s *session) run(ctx context.Context) int {
	devs, err := s.br.Devices(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return s.nothing()
		}
		if errors.Is(err, z2m.ErrNoRetained) {
			err = fmt.Errorf("%w; is Zigbee2MQTT running, with retain on?", err)
		}
		fmt.Fprintln(s.err, err)
		return 1
	}
	offered, refused, others := survey(devs)
	if len(offered) == 0 {
		fmt.Fprintln(s.err, "Zigbee2MQTT has no light that zigbeat may drive")
		return 1
	}
	var existing []config.Fixture
	current := ""
	if c := s.o.existing; c != nil {
		existing, current = c.Fixtures, c.Canary
	}
	proposal, _ := arrange(existing, offered)
	marked := map[string]bool{}
	for _, l := range config.LightsOf(existing) {
		marked[l] = true
	}
	printTable(s.out, len(devs)-1, offered, fixtureOf(proposal), marked, refused, others)
	for _, l := range config.LightsOf(existing) {
		if indexOf(offered, l) < 0 {
			fmt.Fprintf(s.out, "%s: %s; removed from the config\n", l, whyNot(devs, l))
		}
	}
	chosen, code := s.chooseLights(ctx, devs, offered, marked)
	if code != 0 {
		return code
	}
	fixtures, notes := arrange(existing, chosen)
	for _, n := range append(notes, moved(chosen, fixtureOf(proposal), fixtureOf(fixtures))...) {
		fmt.Fprintln(s.out, n)
	}
	canary, code := s.chooseCanary(ctx, devs, chosen, current)
	if code != 0 {
		return code
	}
	if ctx.Err() != nil {
		return s.nothing()
	}
	if err := s.write(fixtures, canary); err != nil {
		fmt.Fprintln(s.err, err)
		return 1
	}
	s.done(fixtures, canary)
	return 0
}

// nothing ends setup after Ctrl-C or the end of the input.
func (s *session) nothing() int {
	fmt.Fprintln(s.out, "\nnothing was written")
	return cli.ExitInterrupted
}
