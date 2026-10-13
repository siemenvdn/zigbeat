// Package cli holds what the zigbeat subcommands share.
package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// StopSignals end a command gracefully, with a restore.
var StopSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}

// StopContext is done at the first stop signal.
func StopContext() (context.Context, context.CancelFunc) {
	var sigs []os.Signal
	for _, s := range StopSignals {
		if s != syscall.SIGHUP || !signal.Ignored(s) {
			sigs = append(sigs, s)
		}
	}
	return signal.NotifyContext(context.Background(), sigs...)
}

// QuitOnInterrupt lets a second Ctrl-C quit at once, without restoring.
func QuitOnInterrupt() { signal.Reset(os.Interrupt) }

// QuitHint says what QuitOnInterrupt allows.
const QuitHint = "Ctrl-C again to quit without restoring"
