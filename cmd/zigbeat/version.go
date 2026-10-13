package main

import "runtime/debug"

const (
	unknownVersion = "dev"
	develVersion   = "(devel)"
)

var version string

func init() {
	info, ok := debug.ReadBuildInfo()
	version = chosenVersion(version, info, ok)
}

func chosenVersion(stamped string, info *debug.BuildInfo, ok bool) string {
	switch {
	case stamped != "":
		return stamped
	case ok && info.Main.Version != "" && info.Main.Version != develVersion:
		return info.Main.Version
	}
	return unknownVersion
}
