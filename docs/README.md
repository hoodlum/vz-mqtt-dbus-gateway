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

## Open findings (2026-10-25)

Found while writing this documentation. **None of them is fixed yet.** The owner hasn't decided whether they go into a v1.0.2 or into the migration. Details, evidence and proposed fixes: [migration-guide.md](migration-guide.md#details-of-the-open-findings).

| # | Severity | Finding | Effect on ESS |
|---|---|---|---|
| K1 | High | The D-Bus value store `victronValues` is read and written concurrently without a lock (confirmed: godbus serves every call in its own goroutine). | Go can abort the process (`fatal error: concurrent map read and map write`). The grid meter is gone until `startup.sh` restarts it after 10 s. |
| K2 | Medium | Invalidation only emits `PropertiesChanged`; the stored values stay. | A consumer that reads `GetValue` after the watchdog fired gets the last real value instead of "invalid". |
| K3 | Medium | Valid JSON with missing fields (e.g. `{}`) counts as a message. | The watchdog is reset and 0 W / 0 kWh are published instead of "invalid". |

Not verified on the Venus (taken from the first handover, 2026-10-10):

- The content of `/data/rc.local`.
- Where the output is redirected to `/data/vz-mqtt-dbus-gateway/vz-mqtt-dbus-gateway.log`. The binary only writes to stdout/stderr, so the redirect is in `rc.local` or in the owner's setup.

Repository housekeeping:

- The root `README.md` describes v0.1.0 (download URL and archive name are outdated) and doesn't link to `docs/`.
- `bin/startup.sh` differs from the one on the Venus (no `--syslog`, no `--publish_statics`).

## Rules that came out of operating it

- Ask the owner before pushing to GitHub (including tags) and before running anything on the Venus.
- Only restart the gateway on the Venus after asking: every restart leaves ESS without a grid meter for about 10–15 seconds.
- Read a file before overwriting it. The owner also edits files on the Venus and in the repo.
- Conventional Commits (`feat:`, `fix:`, `build:`, `docs:` …), several small logical commits.
