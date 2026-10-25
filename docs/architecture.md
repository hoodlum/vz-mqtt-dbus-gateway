# Architecture

## Context

```
┌──────────────┐     ┌───────────────┐   MQTT v5    ┌────────────────────────┐   D-Bus (system)   ┌─────────────────────────┐
│ smart meter  │ ──> │ publisher     │ ───────────> │ vz-mqtt-dbus-gateway   │ ─────────────────> │ Venus OS                │
│ (SML/OBIS)   │     │ (not in repo) │  broker      │ (this service, on the  │  com.victronenergy │ dbus-systemcalc-py      │
└──────────────┘     └───────────────┘  192.168.    │  Venus, armv7)         │  .grid.cgwacs_vzTWO│ hub4control (ESS)       │
                                         178.3:1883  └───────────┬────────────┘                    │ GUI, VRM                │
                                                                 │ UDP syslog (RFC 5424)           └─────────────────────────┘
                                                                 └──────────────> 192.168.178.3:514 (central log store)
```

- **Upstream:** the publisher reads the meter and publishes JSON on `/smartmeter1/power`. It isn't part of this repository. The field names follow the OBIS codes (`1.8.1`, `2.8.0`, `16.7.0`), as volkszaehler does, hence "vz" in the name.
- **Downstream:** Venus OS finds grid meters by the D-Bus service name prefix `com.victronenergy.grid.`. systemcalc aggregates the values, and ESS (hub4control) regulates on `/Ac/Power`. The full interface is in [dbus-contract.md](dbus-contract.md).

## Input message

One JSON object per MQTT message:

```json
{"ts":1791584352000,"energy1_8_1":1234567.8,"energy2_8_0":765432.1,"power16_7_0":-420}
```

| Field | Go field | Meaning | Unit | Used for |
|---|---|---|---|---|
| `ts` | `TimeStamp` | Measurement time, Unix epoch | ms | parsed, **not used** |
| `energy1_8_1` | `GridIn` | Energy imported from the grid (OBIS 1.8.1) | Wh | `/Ac/Energy/Forward` |
| `energy2_8_0` | `GridOut` | Energy exported to the grid (OBIS 2.8.0) | Wh | `/Ac/Energy/Reverse` |
| `power16_7_0` | `ActualPower` | Current total active power, positive = import, negative = export (OBIS 16.7.0) | W, `int32` | `/Ac/Power` and the per-phase values |

Payloads that aren't valid JSON are dropped silently. Valid JSON with missing fields is accepted with zero values; see [migration-guide.md](migration-guide.md#known-issues-and-quirks).

## Components (source files)

| File | Responsibility |
|---|---|
| `main.go` | Flags, logging setup, startup order, dispatcher goroutine, MQTT retry loop, watchdog wiring, signal handling |
| `vz_mqtt_client.go` | MQTT session: dial, connect, subscribe, receive, JSON decode, detect connection loss; `SmartMeterData` and `UnixTime` |
| `victronDbusClient.go` | D-Bus side: path table and initial values, name registration, export of `com.victronenergy.BusItem`, `PropertiesChanged` signals, invalidation |
| `watchdog.go` | Timer that fires when no message arrived within the interval (`time.AfterFunc`, reset per message) |
| `syslog.go` | logrus hook that sends RFC 5424 messages over UDP with microsecond UTC timestamps |
| `version.go` | `version`/`commit`/`date` (set via `-ldflags`), fallback from the embedded build info, `versionString()` |
| `*_test.go` | Tests: MQTT session against a fake broker over `net.Pipe()` (message, connection loss, server disconnect, credentials), syslog format and UDP, version resolution |
| `build.sh` | Manual armv7 build with version ldflags into `dist/` |
| `.goreleaser.yaml`, `.github/workflows/ci.yml` | Release build and CI, see [operations.md](operations.md) |
| `bin/startup.sh` | Example of the restart loop. **Outdated**; the copy on the Venus is the one that's used. |

There is a single `main` package and no internal package structure. State lives in package-level variables (`victronValues`, `basicPaths`, `updatingPaths`, `version`, …).

## Data flow

```
paho read loop ──Router──> msgChan ──> runMqttSession ──json──> messages ──> dispatcher ──> pushSmartmeterData ──> D-Bus signals
 (paho goroutine)        (unbuffered)  (MQTT goroutine)       (unbuffered) (goroutine)   │                       + victronValues
                                                                                         └─> watchdog.ResetWatchdog()

D-Bus GetValue/GetText calls (godbus goroutines) ──> read victronValues
```

Per message, `pushSmartmeterData` sets all 19 dynamic paths:

| Path | Value |
|---|---|
| `/Ac/Power` | `power16_7_0` |
| `/Ac/L1..L3/Power` | `power16_7_0 / 3` (integer division, then float) |
| `/Ac/L1..L3/Current` | `(power / 3) / 230` |
| `/Ac/L1..L3/Voltage` | fixed `230` |
| `/Ac/Energy/Forward`, `/Ac/Energy/Reverse` | `energy1_8_1 / 1000`, `energy2_8_0 / 1000` (kWh) |
| `/Ac/L1..L3/Energy/Forward`, `/Reverse` | the energy split into thirds (kWh) |
| `/Ac/Frequency` | fixed `50` |

Each update stores the value in `victronValues` (answers `GetValue`/`GetText`) and emits `com.victronenergy.BusItem.PropertiesChanged` with `{"Value": v, "Text": "%.2f<unit>"}` on the path.

Both channels are unbuffered. If D-Bus emits are slow, back pressure goes up to the paho read loop. There's no queue and no backlog, so the newest value always wins.

## Goroutines

| Goroutine | Started by | Does |
|---|---|---|
| main | runtime | Setup, then blocks on `signalChan` |
| dispatcher | `main` | `messages` → reset the watchdog → `pushSmartmeterData` |
| MQTT loop | `main` | `startMqttGateway` forever, 5 s pause after each return |
| paho internals | `paho.NewClient`/`Connect` | Socket read/write, pings, router callback into `msgChan`, `OnClientError`/`OnServerDisconnect` |
| watchdog timer | `time.AfterFunc` | After 10 s without a message: `invalidateData`, and with `--force-exit` `os.Exit(1)` |
| godbus handlers | `dbus.SystemBus()` | Serve `GetValue`/`GetText`/`Introspect` for exported paths |

`victronValues` is written by the dispatcher and read by godbus handlers **without a lock**; see the known issues.

## Lifecycle

### Startup order (`main.go`)

1. Parse flags, resolve the version; with `--version`, print it and exit.
2. Set up logging (level, microsecond format), and syslog if `--syslog` is set. A syslog failure only logs a warning.
3. Log `Gateway: starting <versionString>`.
4. Connect to the D-Bus system bus. Failure → `log.Fatal` (exit 1).
5. `initDbus`: build the path table (dynamic paths start **invalid**), request the name `com.victronenergy.grid.cgwacs_vzTWO` with `DoNotQueue`, and export all paths. If the name is taken → `log.Panic` (exit 2). With `--publish_statics`, emit `PropertiesChanged` once for every static path.
6. Start the watchdog (10 s).
7. Start the dispatcher and the MQTT loop.
8. Wait for SIGINT/SIGTERM/SIGQUIT.

D-Bus registration comes before the watchdog so the watchdog can't invalidate paths that aren't registered yet (commit `c8c3075`).

### MQTT session (`runMqttSession`)

1. `startMqttGateway` dials TCP with a 5 s timeout, and the session runs on that connection. `defer conn.Close()`.
2. MQTT v5 CONNECT: `KeepAlive 30`, `CleanStart true`, client ID from `--clientid`, username and password only if set. 5 s timeout.
3. Non-zero CONNACK reason code → log and return.
4. SUBSCRIBE `--topic` with `--qos` (paho's 10 s packet timeout). A granted QoS other than the requested one → log and return.
5. Receive until `done` is closed. `done` closes on the first `OnClientError` (network error, missed pings) or `OnServerDisconnect` (DISCONNECT from the broker).
6. Return → the MQTT loop logs `MQTT gateway stopped, retrying in 5 seconds` and starts over.

### Shutdown

On SIGINT, SIGTERM or SIGQUIT: invalidate all dynamic paths, return from `main` (deferred: cancel the dispatcher context, close the D-Bus connection). The process exit releases the D-Bus name, and Venus OS removes the meter.

## Failure handling

| Failure | Detected by | Reaction | Effect on ESS |
|---|---|---|---|
| Broker down / restart / network loss | paho `OnClientError` (EOF, ping timeout ≤ ~30 s) or `OnServerDisconnect` | Session ends, reconnect after 5 s, repeated until it works | Watchdog invalidates after 10 s without data; values come back with the first new message |
| Broker unreachable at startup | dial or connect timeout (5 s) | Retry every ~5–10 s | Values stay invalid |
| Publisher stops, broker still up | Watchdog (10 s without a message) | Invalidate dynamic paths; with `--force-exit` exit (supervisor restarts) | Invalid grid meter |
| Malformed JSON | `json.Unmarshal` error | Message dropped, watchdog not reset | After 10 s invalid |
| D-Bus not reachable at startup | `dbus.SystemBus()` error | `log.Fatal`, exit 1 → supervisor restarts after 10 s | No meter |
| D-Bus name already taken | `RequestName` reply | `log.Panic`, exit 2 → supervisor restarts after 10 s | The other owner keeps the name |
| Crash / panic | process exit | `startup.sh` restarts after 10 s | No meter for ~10 s; the D-Bus name is released |
| SIGTERM / SIGINT | signal handler | Invalidate, exit | Meter disappears |

There are two restart levels: **in-process** (MQTT reconnect, since v1.0.1) and **supervisor** (`startup.sh` loop with `sleep 10` for crashes and fatal errors).
