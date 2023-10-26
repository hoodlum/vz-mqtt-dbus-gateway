#!/bin/sh
while true; do
  /data/vz-mqtt-dbus-gateway/vz-mqtt-dbus-gateway --server=192.168.178.3:1883 --topic="/smartmeter1/power" --clientid="vz-mqtt-dbus-gateway"
  sleep 10
done
