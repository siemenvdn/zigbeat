package cli

import "time"

// Clock is the time format of every line a command prints.
const Clock = "15:04:05"

// Stamp is the time now in the Clock format.
func Stamp() string { return time.Now().Format(Clock) }
