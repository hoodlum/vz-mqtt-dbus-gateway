package main

import (
	"runtime/debug"
	"testing"
)

func vcsInfo(mainVersion, revision, time, modified string) *debug.BuildInfo {
	return &debug.BuildInfo{
		Main: debug.Module{Version: mainVersion},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: revision},
			{Key: "vcs.time", Value: time},
			{Key: "vcs.modified", Value: modified},
		},
	}
}

func TestResolveVersion(t *testing.T) {
	const rev = "fb1b43bf4c434e8b01d4f9870faa1285c75b5485"
	const ts = "2026-10-09T21:21:08Z"

	cases := []struct {
		name                  string
		ldVersion, ldCommit   string
		info                  *debug.BuildInfo
		version, commit, date string
	}{
		{"ldflags win", "v0.7.0", "abc1234", vcsInfo("v0.7.1-0.2026", rev, ts, "true"), "v0.7.0", "abc1234", ts},
		{"go build on tag", "", "", vcsInfo("v0.7.0", rev, ts, "false"), "v0.7.0", "fb1b43b", ts},
		{"go build untagged", "", "", vcsInfo("(devel)", rev, ts, "false"), "dev-fb1b43b", "fb1b43b", ts},
		{"go build dirty", "", "", vcsInfo("(devel)", rev, ts, "true"), "dev-fb1b43b-dirty", "fb1b43b", ts},
		{"no build info", "", "", nil, "dev", "unknown", "unknown"},
	}

	for _, c := range cases {
		version, commit, date = c.ldVersion, c.ldCommit, ""
		resolveVersion(c.info)
		if version != c.version || commit != c.commit || date != c.date {
			t.Errorf("%s: got (%s, %s, %s), expected (%s, %s, %s)",
				c.name, version, commit, date, c.version, c.commit, c.date)
		}
	}
}
