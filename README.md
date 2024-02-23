"# vz-mqtt-dbus-gateway" 


curl -s -L https://github.com/hoodlum/vz-mqtt-dbus-gateway/releases/download/v0.1.0/vz-mqtt-dbus-gateway_0.1.0_linux_armv7.tar.gz | tar xvz
./vz-mqtt-dbus-gateway --server=192.168.178.3:1883 --topic="/smartmeter1/power" --clientid="vz-mqtt-dbus-gateway"
