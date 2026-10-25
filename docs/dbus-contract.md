# D-Bus contract

This is the interface Venus OS depends on. **A migrated service must produce the same contract**, otherwise ESS loses its grid meter or computes with wrong values. Victron's reference: <https://github.com/victronenergy/venus/wiki/dbus#grid-meter>.

## Service

| | |
|---|---|
| Bus | system bus (the process runs as `root` on the Venus) |
| Service name | `com.victronenergy.grid.cgwacs_vzTWO` |
| Name request | `NameFlagDoNotQueue`; if the name is already owned, the gateway exits instead of waiting |
| Interface on every path | `com.victronenergy.BusItem` (methods `GetValue() → v`, `GetText() → s`; signal `PropertiesChanged(a{sv})`) and `org.freedesktop.DBus.Introspectable` |

The prefix `com.victronenergy.grid.` makes Venus OS treat the service as a grid meter. The suffix `cgwacs_*` mimics Victron's own Carlo Gavazzi meter driver (`dbus-cgwacs`). The code comment says some Victron components require a `grid.cgwacs_*` name and that the name was taken from the simulator as the only known valid value. **Keep the full name and `/DeviceInstance` unchanged**, otherwise Venus OS and VRM may treat the meter as a new device.

## Static paths (identity)

Set once at startup and valid for the whole lifetime, **including while the dynamic values are invalid**.

| Path | Value (`GetValue`) | Text | Note |
|---|---|---|---|
| `/Connected` | `1` | `1` | always 1 |
| `/CustomName` | `Grid meter` | | |
| `/DeviceInstance` | `30` | | hard-coded, not assigned by localsettings |
| `/DeviceType` | `71` | | |
| `/ErrorCode` | `0` | | |
| `/FirmwareVersion` | `2` | | |
| `/Mgmt/Connection` | `/dev/ttyUSB2` | | fake, mimics `dbus-cgwacs` |
| `/Mgmt/ProcessName` | `/opt/color-control/dbus-cgwacs/dbus-cgwacs` | | fake, mimics `dbus-cgwacs` |
| `/Mgmt/ProcessVersion` | gateway version, e.g. `v1.0.1` | | from the git tag; `0.6.0` without `v` up to v0.6.0 |
| `/Position` | `0` | | |
| `/ProductId` | `45058` | | |
| `/ProductName` | `Grid meter` | | |
| `/Serial` | `BP98305081235` | | fixed fake serial |

With `--publish_statics` (set on the Venus), a `PropertiesChanged` signal is emitted once for every static path after registration.

## Dynamic paths (measurements)

Updated on every MQTT message. Value type is `double` (`d`); the text is `fmt.Sprintf("%.2f", v) + unit`, e.g. `-420.00W`.

| Path | Unit | Source |
|---|---|---|
| `/Ac/Power` | W | `power16_7_0`. **ESS regulates on this value.** Positive = import, negative = export. |
| `/Ac/L1/Power`, `/Ac/L2/Power`, `/Ac/L3/Power` | W | `power16_7_0 / 3` |
| `/Ac/L1/Current` … `/Ac/L3/Current` | A | phase power / 230 |
| `/Ac/L1/Voltage` … `/Ac/L3/Voltage` | V | fixed 230 |
| `/Ac/Energy/Forward` | kWh | `energy1_8_1 / 1000` (import) |
| `/Ac/Energy/Reverse` | kWh | `energy2_8_0 / 1000` (export) |
| `/Ac/L1..L3/Energy/Forward` | kWh | import / 3 |
| `/Ac/L1..L3/Energy/Reverse` | kWh | export / 3 |
| `/Ac/Frequency` | Hz | fixed 50 |

The source only delivers the total power and two energy counters. The per-phase values are a symmetric split, and voltage and frequency are constants. See [design-decisions.md](design-decisions.md#d2-symmetric-three-phase-split-fixed-voltage-and-frequency).

## Invalid values

Victron's convention for "no valid value" is an **empty array** (`ai`, `[]int32{}`). The gateway uses it:

- **At startup:** all dynamic paths start invalid (in `victronValues`) until the first message arrives.
- **Watchdog (10 s without a message) and shutdown:** `PropertiesChanged` with `Value = []` and `Text = []` on every dynamic path.
- **Static paths are never invalidated** (like Victron's own services; commit `c8c3075`).

History: `nil` → empty string `""` → empty `int32` array (commits `d2d6736`, `0eae967` "mimic the official victron invalidation of values"). The empty array is the convention of Victron's own services (`vedbus`).

**Known gap:** invalidation only emits the signal. It doesn't overwrite `victronValues`, so a later `GetValue` still returns the last valid value. See [migration-guide.md](migration-guide.md#known-issues-and-quirks).

## Not implemented

- No `GetItems` / `ItemsChanged` on `/` (the newer bulk API of Victron's `vedbus`). Venus OS reads the paths one by one, which works with the current Venus version.
- No `SetValue` (it's in the introspection XML but not implemented): the meter is read-only.
- No `/Ac/L*/PowerFactor` and no `/Ac/Current`.

## Checking it on the Venus

```sh
dbus -y | grep grid                                                   # service present?
dbus -y com.victronenergy.grid.cgwacs_vzTWO /Mgmt/ProcessVersion GetValue
dbus -y com.victronenergy.grid.cgwacs_vzTWO /Ac/Power GetValue        # repeat: the value must change
dbus-spy                                                              # interactive view of all paths
```
