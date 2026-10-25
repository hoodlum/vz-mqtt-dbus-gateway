# Migration guide

This guide is independent of the target (rewrite, another language or library, merge into another service, another host). It lists what a migration has to keep, what it may drop, what it should fix, and how to prove the result before cut-over.

## Must be kept (invariants)

| # | Invariant | Where |
|---|---|---|
| I1 | D-Bus service name `com.victronenergy.grid.cgwacs_vzTWO` on the system bus, interface `com.victronenergy.BusItem` with `GetValue`/`GetText` and `PropertiesChanged` | [dbus-contract.md](dbus-contract.md) |
| I2 | All 13 static paths with the same values (especially `/DeviceInstance 30`, `/ProductId 45058`, `/DeviceType 71`, `/Position 0`); they stay valid while the dynamic values are invalid | [dbus-contract.md](dbus-contract.md#static-paths-identity) |
| I3 | All 19 dynamic paths with the same units, value type `double`, and the same computation from the input (signs: import positive, export negative) | [dbus-contract.md](dbus-contract.md#dynamic-paths-measurements) |
| I4 | Invalid = empty array on value and text; dynamic paths start invalid | [dbus-contract.md](dbus-contract.md#invalid-values) |
| I5 | No stale values: invalidate after at most 10 s without a valid message | D4 |
| I6 | Reconnect to MQTT by itself after any connection loss, without leaving D-Bus | D5 |
| I7 | Never two owners of the meter name: fail when it's taken, don't queue | D6 |
| I8 | Invalidate on shutdown | D7 |
| I9 | Input: MQTT v5, topic `/smartmeter1/power`, the JSON format in [architecture.md](architecture.md#input-message); the publisher isn't changed | |
| I10 | Runs unattended on the Venus (armv7, `/data`, survives firmware updates), restarts after crashes | D10, [operations.md](operations.md#runtime-on-the-venus) |
| I11 | Version traceable to the commit, visible in `/Mgmt/ProcessVersion` and the startup log | D11 |
| I12 | Logs with microsecond timestamps, optional syslog that can't stop the service | D12 |
| I13 | The same command-line flags, or a documented mapping, so `startup.sh` keeps working or gets changed deliberately | [operations.md](operations.md#configuration-flags) |

## Can be dropped or changed freely

- The internal structure (single `main` package, package-level variables).
- The `linux/amd64` release artifact (not used).
- Unused flags and parameters: `TimeStamp` from the input isn't used.
- Dead code: the `0.0` initial values for dynamic paths in `initDbusVariants` (overwritten by "invalid" at the end of the function), commented-out code (`//"vz-mqtt-dbus-gateway/sml/Message"`, logger lines).
- `bin/startup.sh` in the repository (outdated) and the root `README.md` (describes v0.1.0).

## Known issues and quirks

Decide for each one whether the migration fixes it or deliberately keeps it. **Fixing I3-relevant quirks changes values that ESS sees**, so do it in a separate, announced step.

| # | Severity | Issue | Where | Proposal |
|---|---|---|---|---|
| K1 | High | **Data race on `victronValues`:** written by the dispatcher, read by godbus handlers, without a lock. Go can abort with `fatal error: concurrent map read and map write`; the supervisor then restarts after 10 s. | `victronDbusClient.go` | Guard with `sync.RWMutex`, or use godbus `prop`/a single owner goroutine. Verify with `go test -race` (needs CGO). |
| K2 | Medium | **Invalidation doesn't update `victronValues`:** after the watchdog fires, `PropertiesChanged` says invalid, but `GetValue` still returns the last value. | `invalidateData` | Also set the stored value and text to `[]int32{}`. |
| K3 | Medium | **Valid JSON with missing fields counts as a message:** e.g. `{}` resets the watchdog and publishes power 0 and energy 0. | `runMqttSession` | Require the fields (pointer fields or a check), drop the message otherwise. |
| K4 | Low | **Integer division of the phase power:** `float64(ActualPower / 3)`, so `-421 W` gives `-140 W` per phase (sum `-420`). `/Ac/Power` itself is exact. | `pushSmartmeterData` | `float64(ActualPower) / 3`. Changes the per-phase values slightly (I3). |
| K5 | Low | `UnixTime` parses milliseconds wrong: `time.Unix(ts/1000, ts%1000)` uses the remainder as nanoseconds. The value isn't used. | `vz_mqtt_client.go` | `time.UnixMilli(ts)`, or drop the field. |
| K6 | Low | `/Connected` stays `1` while the data is invalid. | static paths | Check what Victron's own meter drivers do before changing it. |
| K7 | Low | `GetValue`/`GetText` of an unknown path return an empty variant instead of an error. Only registered paths are exported, so it can't happen today. | `objectpath` methods | Keep in mind when changing path tables. |
| K8 | Low | `--password` is visible in `ps` (D9). | flags | Environment variable or file if authentication is ever used. |
| K9 | Info | No `GetItems`/`ItemsChanged` on `/`. Works with the current Venus OS; newer Victron services implement it. | | Check against the Venus OS version on the target. |
| K10 | Info | The restart cost is ~10–15 s of no grid meter (`sleep 10` in `startup.sh`). | `startup.sh` | Shorter sleep, or Venus OS's own service supervision instead of the shell loop. |
| K11 | Info | Release tags of this module are v1.x. A v2+ tag needs the module path suffix `/v2` for the build-info version fallback. | `go.mod` | Only relevant without ldflags. |

## Acceptance tests before cut-over

Automated (existing tests, extend for the new implementation):

- MQTT session against a fake broker: message forwarded, return on connection loss, return on server DISCONNECT, credentials only when set (`vz_mqtt_client_test.go`).
- Syslog format and UDP delivery (`syslog_test.go`); version resolution (`version_test.go`).
- **Add:** the D-Bus path table and computations for a fixed input (golden values for all 32 paths, including invalid states).

On the Venus (or a Venus OS test device), compare the old and the new service with the same input:

1. `dbus-spy`, or `dbus -y com.victronenergy.grid.cgwacs_vzTWO <path> GetValue` for every path: same paths, same types, same static values. (`/` itself isn't exported, see K9.)
2. `/Ac/Power` follows the meter; the sign matches import/export in the GUI.
3. Stop the publisher (or block the topic): after ≤ 10 s, all dynamic paths are invalid; static paths stay valid; the GUI shows the meter without values.
4. Restart the broker: `connection lost`, reconnect after 5 s, values come back without restarting the process.
5. Kill the process: `startup.sh` restarts it; ESS recovers (hub4control may restart once, which is normal).
6. Start a second instance: it must exit with "name already taken" and must not disturb the first one.
7. SIGTERM: values are invalid before the name disappears.
8. Reboot the Venus: the service comes up by itself via `/data/rc.local`.
9. Run it for at least a day with long-term monitoring (syslog): no restart loops, no unexplained `Watchdog: triggered`.

## Cut-over plan

1. **Prepare:** build and verify the new binary (checksum), keep the current one as `*.bak`, write down the exact rollback command.
2. **Announce:** the owner agrees on the time. ESS will have no grid meter for a short time.
3. **Switch:** only one service may own the name (I7). Stop the old one completely (including its supervisor loop) before the new one starts, or swap the binary behind the same `startup.sh`.
4. **Verify:** health checks from [operations.md](operations.md#health-checks), then acceptance tests 2–4.
5. **Roll back** at the first deviation in `/Ac/Power` or ESS behaviour: restore `*.bak`, kill the process, verify again.
6. **Clean up** after a stable observation period: remove the backups and update `startup.sh` and these docs.

## Things outside this repository

| Item | Where | Note |
|---|---|---|
| Meter publisher | unknown host, publishes to `/smartmeter1/power` | defines the input format |
| MQTT broker | `192.168.178.3:1883` | must support MQTT v5 |
| Log store | `192.168.178.3:514` (UDP syslog) | e.g. VictoriaLogs |
| `/data/rc.local`, `/data/vz-mqtt-dbus-gateway/startup.sh` | Venus | the effective runtime configuration |
| `deploy-armv7.bat` | local, gitignored | contains the SSH key path |
| The owner's update script | owner's machine | used to deploy v1.0.1 |
