# Changelog

## v0.7.0 (2026-10-10)

### Fixes
- Syslog messages are sent as RFC 5424 with microsecond UTC timestamps. Go's `log/syslog` only sent whole seconds, so the syslog server couldn't keep the order of entries within one second.
- The local log output uses microsecond timestamps as well.
- `--syslog` now also works on Windows.

### Features
- `--version` and the startup log line print version, commit, build date, Go version and platform. The version comes from the git tag at build time (`build.sh`, GoReleaser) and is published as `/Mgmt/ProcessVersion` on D-Bus, now with the leading `v` (e.g. `v0.7.0`, was `0.6.0`).

### Security
- logrus v1.10.2 (GHSA-4f99-4q7p-p3gh, DoS in `Entry.Writer()`; the gateway doesn't call it) and golang.org/x/sys v0.49.0.

### Build
- Releases are built with the latest Go 1.26 patch release (was Go 1.20). Minimum Go version is 1.26.0 (required by golang.org/x/sys).
- GoReleaser config migrated to v2. The CI ran GoReleaser v1 before.
- Release Docker images (`ghcr.io/hoodlum/vz-mqtt-dbus-gateway:<version>`, `-amd64`, `-armv7`, `latest`) are now pushed. Before, the release job only built them.
- Tests run before the build in CI.
- `build.sh` builds a versioned armv7 binary for manual deployments (`git describe`, e.g. `v0.7.0-2-gabc1234-dirty`).
