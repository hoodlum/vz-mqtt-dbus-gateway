# vz-mqtt-dbus-gateway: handover documentation

*Status: v1.0.1, 2026-10-25. Written for a migration of the service. Covers this service only.*

## What the service does

The gateway reads smart meter values from an MQTT topic and publishes them on the Venus OS D-Bus as a Victron grid meter (`com.victronenergy.grid.cgwacs_vzTWO`). The Victron ESS control loop uses this grid meter to regulate grid import and export.

```
smart meter ──> (publisher, not in this repo) ──MQTT──> vz-mqtt-dbus-gateway ──D-Bus──> Venus OS (systemcalc, hub4control/ESS, GUI, VRM)
```

**The service is critical for ESS.** Without a valid grid meter, ESS can't regulate, and hub4control restarts itself until a meter is back. Every change and every migration step must keep that in mind.

## Documents

| Document | Content |
|---|---|
| [architecture.md](architecture.md) | Components, data flow, goroutines, lifecycle, failure handling, source files |
| [dbus-contract.md](dbus-contract.md) | The D-Bus interface Venus OS depends on: service name, paths, values, units, invalidation |
| [design-decisions.md](design-decisions.md) | Design decisions and principles, with the reason behind each one |
| [operations.md](operations.md) | Configuration, build, release, deployment on the Venus, logs, health checks, rollback |
| [migration-guide.md](migration-guide.md) | What a migration must keep, known issues and quirks, acceptance tests, cut-over plan |

[`../CHANGELOG.md`](../CHANGELOG.md) has the release history since v1.0.0.

## Key facts

| | |
|---|---|
| Repository | `git@github.com:hoodlum/vz-mqtt-dbus-gateway.git`, branch `main` |
| Language | Go 1.26, module `vz-mqtt-dbus-gateway`, single static binary (`CGO_ENABLED=0`) |
| Target | Venus OS on a Raspberry Pi, `linux/arm` v7 (host `venus.local`) |
| Input | MQTT v5, broker `192.168.178.3:1883`, topic `/smartmeter1/power`, JSON |
| Output | D-Bus system bus, service `com.victronenergy.grid.cgwacs_vzTWO`, interface `com.victronenergy.BusItem` |
| Logs | stdout/stderr (logrus text, microseconds) and optionally RFC 5424 syslog over UDP (`192.168.178.3:514`) |
| Runtime | `/data/rc.local` → `/data/vz-mqtt-dbus-gateway/startup.sh` (restart loop) → binary |
| Release | Git tag `vX.Y.Z` → GitHub Actions → GoReleaser v2 → GitHub release with tarballs and `checksums.txt` |

## Rules that came out of operating it

- Ask the owner before pushing to GitHub (including tags) and before running anything on the Venus.
- Only restart the gateway on the Venus after asking: every restart leaves ESS without a grid meter for about 10–15 seconds.
- Read a file before overwriting it. The owner also edits files on the Venus and in the repo.
- Conventional Commits (`feat:`, `fix:`, `build:`, `docs:` …), several small logical commits.
