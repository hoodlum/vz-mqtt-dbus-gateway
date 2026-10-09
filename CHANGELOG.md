# Changelog

## v1.0.1 (2026-10-10)

### Fixes
- The gateway reconnects to the MQTT broker after the connection is lost (broker restart, network error, missed pings). Before, it stayed disconnected until it was restarted by hand, unless `--force-exit` was set, so ESS ran without a grid meter.
- `--username` / `--password` are now sent to the broker. Before, only the flags were set and the values were empty.
- No crash when the broker rejects the connection without a reason string.

## v1.0.0 (2026-10-10)

### Fixes
- Syslog messages are sent as RFC 5424 with microsecond UTC timestamps. Go's `log/syslog` only sent whole seconds, so the syslog server couldn't keep the order of entries within one second.
- The local log output uses microsecond timestamps as well.
- `--syslog` now also works on Windows.

### Features
- `--version` and the startup log line print version, commit, build date, Go version and platform. The version comes from the git tag at build time (`build.sh`, GoReleaser) and is published as `/Mgmt/ProcessVersion` on D-Bus, now with the leading `v` (e.g. `v1.0.0`, was `0.6.0`).

### Security
- logrus v1.10.2 (GHSA-4f99-4q7p-p3gh, DoS in `Entry.Writer()`; the gateway doesn't call it) and golang.org/x/sys v0.49.0.

### Build
- Releases are built with the latest Go 1.26 patch release (was Go 1.20). Minimum Go version is 1.26.0 (required by golang.org/x/sys).
- GoReleaser config migrated to v2. The CI ran GoReleaser v1 before.
- No more Docker images. Releases attach the `linux/armv7` and `linux/amd64` tarballs and `checksums.txt`. The release job only built the images and never pushed them, and the CI no longer pushes branch snapshot images to ghcr.io.
- Tests run before the build in CI.
- `build.sh` builds a versioned armv7 binary for manual deployments (`git describe`, e.g. `v1.0.0-2-gabc1234-dirty`).
