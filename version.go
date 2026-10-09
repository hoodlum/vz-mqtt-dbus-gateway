package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// Set at build time, see build.sh and .goreleaser.yaml:
//
//	-ldflags "-X main.version=v1.0.0 -X main.commit=fb1b43b -X main.date=2026-10-09T21:31:00Z"
var (
	version = ""
	commit  = ""
	date    = ""
)

// resolveVersion fills version, commit and date that were not set via -ldflags
// from the build info embedded by `go build` (git tag, revision, commit time).
func resolveVersion(info *debug.BuildInfo) {
	modified := false

	if info != nil {
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				if commit == "" {
					commit = s.Value
				}
			case "vcs.time":
				if date == "" {
					date = s.Value
				}
			case "vcs.modified":
				modified = s.Value == "true"
			}
		}

		if version == "" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}

	if len(commit) > 7 {
		commit = commit[:7]
	}

	if version == "" {
		version = "dev"
		if commit != "" {
			version += "-" + commit
		}
		if modified {
			version += "-dirty"
		}
	}

	if commit == "" {
		commit = "unknown"
	}
	if date == "" {
		date = "unknown"
	}
}

func versionString() string {
	return fmt.Sprintf("vz-mqtt-dbus-gateway %s (commit %s, built %s, %s, %s/%s)",
		version, commit, date, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
