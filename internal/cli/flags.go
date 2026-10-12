package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

// ErrUsage means the flag package already printed the problem and the usage.
var ErrUsage = errors.New("usage")

// Exit codes.
const (
	ExitUsage       = 2   // a bad flag, as the flag package uses it
	ExitInterrupted = 130 // 128 + SIGINT, as shells report a Ctrl-C
)

// NewFlagSet returns the flag set of a subcommand; -h prints usage, about and the flags.
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

// Exit turns an error from parsing and checking flags into an exit code: 0 after
// -h, ExitUsage for a bad flag. Other errors are printed. stop is false for nil.
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
