#!/bin/sh
# Build the Venus OS binary (Raspberry Pi, linux/arm v7) for a manual deployment.
# Release builds are done by GoReleaser in CI, see .goreleaser.yaml.
set -eu

VERSION=$(git describe --tags --always --dirty)
COMMIT=$(git rev-parse --short HEAD)
DATE=$(TZ=UTC0 git log -1 --date=format-local:%Y-%m-%dT%H:%M:%SZ --format=%cd)

GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -trimpath \
	-ldflags "-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.date=$DATE" \
	-o dist/vz-mqtt-dbus-gateway .

echo "built dist/vz-mqtt-dbus-gateway $VERSION"
