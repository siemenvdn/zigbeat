package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
)

// ErrUsage means the flag package already printed the problem and the usage.
var ErrUsage = errors.New("usage")

// Exit codes.
const (
	ExitUsage       = 2
	ExitInterrupted = 130
)

// NewFlagSet returns the flag set of a subcommand.
func NewFlagSet(name, usage, about string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), usage)
		fmt.Fprintln(fs.Output(), "\n"+about)
		fmt.Fprintln(fs.Output())
		fs.PrintDefaults()
	}
	return fs
}

// Parse parses args and returns nil, flag.ErrHelp or ErrUsage.
func Parse(fs *flag.FlagSet, args []string) error {
	err := fs.Parse(args)
	if err == nil || err == flag.ErrHelp {
		return err
	}
	return ErrUsage
}

// Exit turns an error from parsing and checking flags into an exit code.
func Exit(err error) (code int, stop bool) {
	switch {
	case err == nil:
		return 0, false
	case err == flag.ErrHelp:
		return 0, true
	case err != ErrUsage:
		fmt.Fprintln(os.Stderr, err)
	}
	return ExitUsage, true
}

// Names splits a comma-separated list of names from a flag and drops empty ones and repeats.
func Names(list string) []string {
	var out []string
	for _, n := range strings.Split(list, ",") {
		if n = strings.TrimSpace(n); n != "" && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

// Fail prints err on stderr and returns exit code 1.
func Fail(err error) int {
	fmt.Fprintln(os.Stderr, err)
	return 1
}
