// zigbeat makes Zigbee lights move with music without flooding the network.
package main

import (
	"fmt"
	"os"

	"github.com/siemenvdn/zigbeat/internal/beats"
	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/probe"
	"github.com/siemenvdn/zigbeat/internal/run"
	"github.com/siemenvdn/zigbeat/internal/serve"
	"github.com/siemenvdn/zigbeat/internal/setup"
)

var version = "dev"

const usage = `zigbeat: make your Zigbee lights move with music, without flooding your network

usage: zigbeat <command> [flags]

commands:
  setup     find your lights in Zigbee2MQTT and write the configuration
  probe     measure how many light commands per second your network handles
  run       run the light show to the music, or to a metronome
  serve     run the show as a service that Home Assistant switches on and off
  beats     print the beats from Music Assistant, without touching a light
  version   print the version

Run "zigbeat <command> -h" for the flags of a command.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(cli.ExitUsage)
	}
	switch os.Args[1] {
	case "setup":
		os.Exit(setup.Run(os.Args[2:]))
	case "probe":
		os.Exit(probe.Run(os.Args[2:]))
	case "run":
		os.Exit(run.Run(os.Args[2:]))
	case "serve":
		os.Exit(serve.Run(os.Args[2:], version))
	case "beats":
		os.Exit(beats.Run(os.Args[2:]))
	case "version", "-v", "--version":
		fmt.Println(version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(cli.ExitUsage)
	}
}
