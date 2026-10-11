package cli

import (
	"os"
	"slices"
	"syscall"
	"testing"
)

func TestStopSignalsIncludeHangup(t *testing.T) {
	// A closed SSH session must restore the lights like Ctrl-C does.
	if !slices.Contains(StopSignals, os.Signal(syscall.SIGHUP)) || !slices.Contains(StopSignals, os.Signal(syscall.SIGTERM)) {
		t.Fatalf("StopSignals = %v", StopSignals)
	}
}
