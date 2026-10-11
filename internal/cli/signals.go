// Package cli holds what the zigbeat subcommands share.
package cli

import (
	"os"
	"syscall"
)

// StopSignals end a command gracefully, with a restore: Ctrl-C, a container
// stop and a closed SSH session.
var StopSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
