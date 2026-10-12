package cli

import (
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
