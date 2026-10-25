# Design decisions and principles

## Principles

1. **ESS safety comes first. An invalid value is better than a wrong or stale one.** ESS regulates the inverter on `/Ac/Power`. A frozen value makes ESS regulate against a reality that no longer exists; an invalid value makes Venus OS fall back to its safe behaviour. Every failure path ends in "invalidate" or "exit and let the supervisor restart".
2. **Look like a Victron device, follow Victron's conventions.** Service name, paths, value types, invalidation with an empty array, static identity paths that stay valid. Venus OS then needs no configuration and no patches.
3. **Self-healing without manual intervention.** The gateway runs unattended. Connection loss, crashes and boot order must resolve themselves (in-process reconnect plus the supervisor loop).
4. **One small static binary, no runtime dependencies.** Venus OS is a minimal embedded Linux; anything beyond a binary in `/data` is a maintenance burden across firmware updates.
5. **Observability with exact timing.** Microsecond timestamps locally and in syslog, so events can be ordered across services in the central log store.
6. **Reproducible, traceable builds.** The version comes from the git tag, never from the source code; every binary tells you which commit it came from.
7. **Minimal, careful changes on the production system.** Changes on the Venus are announced, approved and reversible (backup, rollback).

## Decisions

### D1: Emulate a Victron `cgwacs` grid meter on D-Bus
- **Decision:** register as `com.victronenergy.grid.cgwacs_vzTWO` with the identity of Victron's `dbus-cgwacs` driver (`/Mgmt/ProcessName`, `/Mgmt/Connection`, `/ProductId 45058`, `/DeviceType 71`).
- **Why:** Venus OS (systemcalc, ESS, GUI, VRM) accepts the meter like a supported energy meter, without patches to Venus OS. The service name was taken from the Victron simulator, the only value known to work (code comment).
- **Consequence:** the name and the identity paths are part of the contract ([dbus-contract.md](dbus-contract.md)). `/DeviceInstance` is hard-coded to 30 instead of being assigned by localsettings.

### D2: Symmetric three-phase split, fixed voltage and frequency
- **Decision:** `/Ac/L1..L3/Power` = total / 3, current = phase power / 230 V, voltage = 230 V, frequency = 50 Hz.
- **Why:** the meter (via the publisher) only provides total power and two energy counters. Venus OS expects per-phase paths for a three-phase meter, and ESS only regulates on the total `/Ac/Power`.
- **Consequence:** the per-phase values in the GUI/VRM are estimates, not measurements. If the source ever delivers per-phase values, use them.

### D3: Invalidation like Victron: empty array, dynamic paths only
- **Decision:** invalid = `[]int32{}` on value and text; only the 19 dynamic paths are invalidated; the static paths stay valid.
- **Why:** this is what Victron's own services do (`vedbus`); the earlier `nil` and `""` were replaced to match it. Static identity paths stay valid as in Victron's services, so the device stays known while it has no data.
- **History:** `d2d6736`, `0eae967`, `c8c3075`.

### D4: Watchdog: invalidate after 10 seconds without data, exit only on request
- **Decision:** a 10 s timer, reset by every decoded message. On expiry, invalidate. `--force-exit` additionally exits with status 1.
- **Why:** principle 1. Until v1.0.0, `--force-exit` was the workaround for the missing MQTT reconnect. Since v1.0.1 the process reconnects itself, so exiting is no longer needed and the Venus runs without `--force-exit`.
- **Consequence:** the watchdog also catches "broker up, publisher silent", which the MQTT layer can't detect.

### D5: Two restart levels: in-process reconnect plus an external supervisor loop
- **Decision:** MQTT problems end only the session and reconnect after 5 s (v1.0.1). Everything fatal (D-Bus, name conflict, panic) exits the process, and `startup.sh` restarts it after 10 s.
- **Why:** reconnecting in-process keeps the D-Bus registration, so Venus OS sees "invalid", not "meter gone". The shell loop is the simplest robust supervisor available on Venus OS and survives firmware updates in `/data`.
- **History:** `8a5db1e` (zombie fix with timeouts), `abdd58f` (reconnect on `OnClientError`/`OnServerDisconnect`). Before that, the dispatcher blocked forever after a broker restart.

### D6: Fail fast on D-Bus problems
- **Decision:** no D-Bus connection → `log.Fatal`. The name is taken → `log.Panic`. `RequestName` uses `DoNotQueue`.
- **Why:** without D-Bus the gateway has no purpose. Two processes must never compete for the meter name (e.g. the release binary and a `-debug` binary), and queueing would hide that.

### D7: Graceful shutdown invalidates
- **Decision:** on SIGINT/SIGTERM/SIGQUIT, invalidate all dynamic values, then exit.
- **Why:** consumers learn immediately that the values are gone, instead of keeping the last value until the name disappears.

### D8: MQTT v5 with `eclipse/paho.golang`, clean start, QoS 0, latest value wins
- **Decision:** `paho.golang` (MQTT v5 only), `CleanStart: true`, QoS from `--qos` (default 0), unbuffered channels, no queue.
- **Why:** only the current value matters for regulation; old values are worthless after a reconnect. No session state means no surprises after a broker restart.
- **Consequence:** **the broker must support MQTT v5.** A migration to another client library must keep connection-loss detection (keep-alive 30 s, callbacks) and the reconnect.

### D9: Configuration via command-line flags only
- **Decision:** all settings are flags; only `LOG_LEVEL` comes from the environment (as the flag's default). No config file.
- **Why:** the complete configuration is visible in one line in `startup.sh` and in `ps`. Nothing else has to be kept in sync on the Venus.
- **Consequence:** `--password` is visible in `ps` (not used today: the broker runs without authentication).

### D10: Single static Go binary in `/data`, no container
- **Decision:** `CGO_ENABLED=0`, `-trimpath -s -w`, `linux/arm v7`, deployed to `/data/vz-mqtt-dbus-gateway/`. Docker images were removed in v1.0.0.
- **Why:** Venus OS has no container runtime, and `/data` is the only location that survives firmware updates. The release Docker images were never pushed anyway.
- **Consequence:** releases ship tarballs for `linux_armv7` (used) and `linux_amd64` (not used, kept at no cost).

### D11: Version from the git tag, not from the source code
- **Decision:** `version`/`commit`/`date` are lowercase package variables, set via `-ldflags` (GoReleaser: `{{ .Tag }}` with `v`; `build.sh`: `git describe`). Without ldflags, `resolveVersion` falls back to the build info (module version from the tag, otherwise `dev-<commit>[-dirty]`). The version appears in `--version`, the startup log and `/Mgmt/ProcessVersion`.
- **Why:** one source of truth (the tag); no forgotten version bumps; a running binary can always be traced to its commit. Lowercase variables match GoReleaser's default ldflags.
- **Consequence:** a v2+ tag would need the module path suffix `/v2` for the build-info fallback; with ldflags it works anyway.

### D12: Own RFC 5424 syslog hook with microsecond timestamps
- **Decision:** `syslog.go` sends `<PRI>1 TIMESTAMP HOST vz-mqtt-dbus-gateway PID - - MSG` over UDP, facility daemon, severity mapped like `logrus/hooks/syslog`; logrus fields appended as sorted `key=value`. The local text log uses `2006-01-02T15:04:05.000000Z07:00`. If syslog can't be set up, only a warning is logged.
- **Why:** Go's `log/syslog` (RFC 3164) only has second precision, so the log store couldn't order entries within one second. The hook also works on Windows. A logging failure must never stop the meter.

### D13: Release pipeline: GoReleaser v2, pinned Go, tests first
- **Decision:** CI runs `go test` and `build.sh` on every push; `v*` tags run GoReleaser v2 (`goreleaser-action@v6`, `~> v2`) with Go pinned to `1.26` (latest patch).
- **Why:** `setup-go` with `go.mod` would use the `go` directive (too old), and `goreleaser-action@v5` with `latest` silently used GoReleaser v1. Pinning makes the toolchain explicit.
