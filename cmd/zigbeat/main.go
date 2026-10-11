// zigbeat makes Zigbee lights move with music without flooding the network.
package main

import (
	"fmt"
	"os"

	"github.com/siemenvdn/zigbeat/internal/probe"
	"github.com/siemenvdn/zigbeat/internal/run"
)

var version = "dev"

const usage = `zigbeat: make your Zigbee lights move with music, without flooding your network

usage: zigbeat <command> [flags]

commands:
  probe     measure how many light commands per second your network handles
  run       run the light show to a metronome
  version   print the version

Run "zigbeat <command> -h" for the flags of a command.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "probe":
		os.Exit(probe.Run(os.Args[2:]))
	case "run":
		os.Exit(run.Run(os.Args[2:]))
	case "version", "-v", "--version":
		fmt.Println(version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}
