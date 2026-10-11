// Package cli holds what the zigbeat subcommands share.
package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// StopSignals end a command gracefully, with a restore: Ctrl-C, a container
// stop and a closed SSH session.
var StopSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}

// StopContext is done at the first stop signal. A hangup that was ignored at
// start (nohup) stays ignored.
func StopContext() (context.Context, context.CancelFunc) {
	var sigs []os.Signal
	for _, s := range StopSignals {
		if s != syscall.SIGHUP || !signal.Ignored(s) {
			sigs = append(sigs, s)
		}
	}
	return signal.NotifyContext(context.Background(), sigs...)
}

// QuitOnInterrupt lets a second Ctrl-C quit at once, without restoring. SIGTERM
// and SIGHUP stay caught: a closed SSH session sends a hangup twice, and the
// second one must not cut the restore short.
func QuitOnInterrupt() { signal.Reset(os.Interrupt) }
