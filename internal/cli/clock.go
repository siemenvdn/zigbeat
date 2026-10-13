package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// Clock is the time format of every line a command prints.
const Clock = "15:04:05"

// ClockMS is Clock with milliseconds, for logs of single commands and beats.
const ClockMS = Clock + ".000"

// Stamp is the time now in the Clock format.
func Stamp() string { return time.Now().Format(Clock) }

// Seconds prints d as output lines do: "5 s", "1.5 s".
func Seconds(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', -1, 64) + " s"
}

// Say prints a line on stdout after the time stamp, as every running command does.
func Say(format string, args ...any) {
	fmt.Printf("%s  %s\n", Stamp(), fmt.Sprintf(format, args...))
}

// Sleep waits d, or less when ctx is done; it reports whether ctx is still live.
func Sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(max(0, d))
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
