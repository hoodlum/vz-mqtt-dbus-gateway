# Operations

## Configuration (flags)

| Flag | Default | On the Venus | Meaning |
|---|---|---|---|
| `--server` | `192.168.178.3:1883` | `192.168.178.3:1883` | MQTT broker `host:port` (TCP, no TLS) |
| `--topic` | `/smartmeter1/power` | `/smartmeter1/power` | Topic to subscribe to |
| `--qos` | `0` | default | Subscription QoS |
| `--clientid` | `vz-mqtt-dbus-bridge` | `vz-mqtt-dbus-gateway` | MQTT client ID. Must be unique on the broker; a second client with the same ID kicks the first one off. |
| `--username`, `--password` | empty | not set | Broker credentials, only sent when set (fixed in v1.0.1) |
| `--publish_statics` | `false` | `true` | Emit `PropertiesChanged` once for every static D-Bus path after startup |
| `--force-exit` | `false` | not set | Exit with status 1 when the watchdog fires. Not needed since v1.0.1. |
| `--syslog` | empty | `192.168.178.3:514` | Forward logs as RFC 5424 over UDP |
| `--log-level` | `$LOG_LEVEL`, else `info` | default | `debug`, `info`, `warn`, `error`, `fatal`, `panic`; invalid values fall back to `info` |
| `--version` | | | Print version, commit, build date, Go version and platform, then exit |

## Runtime on the Venus

| | |
|---|---|
| Host | `venus.local`, SSH as `root` with the key `c:\Work.001\Cloud\sst77\sst77.id_rsa` |
| Directory | `/data/vz-mqtt-dbus-gateway/` (`/data` survives Venus OS firmware updates) |
| Binary | `vz-mqtt-dbus-gateway` (release), optionally `vz-mqtt-dbus-gateway-debug` (from `deploy-armv7.bat`) |
| Start chain | `/data/rc.local` (run at boot by Venus OS) → `/data/vz-mqtt-dbus-gateway/startup.sh` → binary |
| Supervisor | `startup.sh`: `while true; do <binary> <flags>; sleep 10; done` |
| Local log | `/data/vz-mqtt-dbus-gateway/vz-mqtt-dbus-gateway.log` (output redirect outside the binary) |
| Remote log | syslog UDP `192.168.178.3:514`, app name `vz-mqtt-dbus-gateway` |

The `startup.sh` on the Venus (as of 2026-10-10; the copy in `bin/` is outdated):

```sh
#tr!/bin/sh

while true; do
  #export GOMEMLIMIT="20MiB"
  #echo $GOMEMLIMIT
  #export LOG_LEVEL="debug"
  /data/vz-mqtt-dbus-gateway/vz-mqtt-dbus-gateway --server=192.168.178.3:1883 --syslog=192.168.178.3:514 --topic="/smartmeter1/power" --clientid="vz-mqtt-dbus-gateway" --publish_statics=true
  sleep 10
done
```

The first line is a broken shebang (`#tr!`). That's harmless because the script is started with `sh`. Read `/data/rc.local` and `startup.sh` on the Venus before changing them; the owner edits them directly.

## Build

| Purpose | Command | Result |
|---|---|---|
| Tests | `go test ./...` | |
| Vet (host and target) | `go vet ./...` and `GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go vet ./...` | |
| Manual armv7 build | `sh build.sh` | `dist/vz-mqtt-dbus-gateway`, version from `git describe` (e.g. `v1.0.1-2-gabc1234-dirty`) |
| Local debug deploy | `deploy-armv7.bat` (gitignored, local only) | builds **without** ldflags → version `dev-<commit>`, copies to `/data/vz-mqtt-dbus-gateway/vz-mqtt-dbus-gateway-debug` |
| Vulnerability check | `go install golang.org/x/vuln/cmd/govulncheck@latest`, then `GOOS=linux GOARCH=arm GOARM=7 govulncheck ./...` | Install it natively first; `go run` with `GOOS=linux` produces a binary that can't run on Windows |

`.gitattributes` forces LF for `*.sh`; otherwise `build.sh` breaks in Windows checkouts.

## CI and release

- `.github/workflows/ci.yml`
  - Job `build` on every push and PR: Go `1.26`, `go test -v ./...`, `sh build.sh`.
  - Job `release` on `v*` tags (after `build`): GoReleaser v2 → GitHub release with `vz-mqtt-dbus-gateway_<version>_linux_armv7.tar.gz`, `..._linux_amd64.tar.gz`, `checksums.txt`.
- `.github/workflows/gitleaks.yml`: secret scan on every push and PR.
- `.goreleaser.yaml`: ldflags `-s -w -X main.version={{ .Tag }} -X main.commit={{ .ShortCommit }} -X main.date={{ .CommitDate }}`, `-trimpath`, `mod_timestamp` from the commit (reproducible).

### Release steps

1. Run the checks: tests, vet (both), `goreleaser check`, `govulncheck`.
2. Run a dry run in a throwaway clone with a local test tag: `goreleaser release --clean --skip=publish`. Install GoReleaser with `GOBIN=<temp dir> go install github.com/goreleaser/goreleaser/v2@latest`.
3. Add a section to `CHANGELOG.md` (format: `## vX.Y.Z (date)`, subsections `Fixes`, `Features`, `Security`, `Build`, `Breaking changes`) and commit it.
4. Create the tag with the changelog section as the message: `git tag -a vX.Y.Z --cleanup=verbatim -F <file>`. Without `--cleanup=verbatim`, git drops the Markdown headings (lines starting with `#`).
5. After approval: `git push origin main vX.Y.Z`, then `gh run watch`.
6. Download the armv7 tarball and verify it with `sha256sum -c --ignore-missing checksums.txt`.

SemVer: removing or renaming a flag, or changing the D-Bus contract, is a breaking change (major bump).

## Deployment on the Venus

Only after approval. ESS has no grid meter for about 10–15 s.

```sh
# local: verified binary from the release tarball
scp -i <key> vz-mqtt-dbus-gateway root@venus.local:/data/vz-mqtt-dbus-gateway/vz-mqtt-dbus-gateway.new

# on the Venus
cd /data/vz-mqtt-dbus-gateway
echo "<sha256>  vz-mqtt-dbus-gateway.new" | sha256sum -c
chmod +x vz-mqtt-dbus-gateway.new && ./vz-mqtt-dbus-gateway.new --version
cp -p vz-mqtt-dbus-gateway vz-mqtt-dbus-gateway.<old-version>.bak
mv vz-mqtt-dbus-gateway.new vz-mqtt-dbus-gateway
kill $(pidof vz-mqtt-dbus-gateway)     # startup.sh restarts the new binary after 10 s
```

The owner also has their own update script, which isn't in the repository. In Claude Code's auto mode, remote commands that change files on the Venus are blocked; the owner runs them.

**Rollback:** `mv vz-mqtt-dbus-gateway.<old-version>.bak vz-mqtt-dbus-gateway && kill $(pidof vz-mqtt-dbus-gateway)`.

## Health checks

```sh
ps | grep [v]z-mqtt                                                       # binary and startup.sh running
tail -20 /data/vz-mqtt-dbus-gateway/vz-mqtt-dbus-gateway.log
dbus -y com.victronenergy.grid.cgwacs_vzTWO /Mgmt/ProcessVersion GetValue # expected version
dbus -y com.victronenergy.grid.cgwacs_vzTWO /Ac/Power GetValue            # twice, a few seconds apart: must change
```

Normal startup log (the goroutine lines can appear in a different order):

```
Gateway: starting vz-mqtt-dbus-gateway v1.0.1 (commit f60b885, built 2026-10-09T22:37:53Z, go1.26.8, linux/arm)
DBUS: connected to Systembus
DBUS: Names = [... com.victronenergy.grid.cgwacs_vzTWO]
DBUS: Registered as a meter
Watchdog: Started
Gateway: Starting MQTT gateway
MQTT: Connected to 192.168.178.3:1883
MQTT: Subscribed to /smartmeter1/power, starting Dispatcher
```

Expected sequence after a broker restart (since v1.0.1):

```
MQTT: connection lost: EOF                              (or a ping timeout, up to ~30 s)
Gateway: MQTT gateway stopped, retrying in 5 seconds
Watchdog: triggered, marking data as invalid            (after 10 s without data)
DBUS: Invalidating data
Gateway: Starting MQTT gateway
MQTT: Connected to 192.168.178.3:1883
```

## Troubleshooting

| Symptom | Likely cause | Check |
|---|---|---|
| `Watchdog: triggered` repeats, no `connection lost` | Publisher silent, or the topic or format changed | `mosquitto_sub -h 192.168.178.3 -t /smartmeter1/power -v` |
| `name cgwacs_* already taken`, restart loop | A second instance is running (e.g. the `-debug` binary) | `ps`, `dbus -y \| grep grid` |
| Repeated `connection lost` right after connecting | Duplicate MQTT client ID | Broker log, `--clientid` |
| The gateway goes quiet without `connection lost` | Connection loss not detected | Report it: this case is expected to be handled since v1.0.1 |
| hub4control restarts | Grid meter missing or invalid | Gateway log, `dbus -y` |
