package main

import (
	"fmt"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
)

type objectpath string

const intro = `
<node>
   <interface name="com.victronenergy.BusItem">
    <signal name="PropertiesChanged">
      <arg type="a{sv}" name="properties" />
    </signal>
    <method name="SetValue">
      <arg direction="in"  type="v" name="value" />
      <arg direction="out" type="i" />
    </method>
    <method name="GetText">
      <arg direction="out" type="s" />
    </method>
    <method name="GetValue">
      <arg direction="out" type="v" />
    </method>
    </interface>` + introspect.IntrospectDataString + `</node> `

var basicPaths, updatingPaths []dbus.ObjectPath

var victronValues = map[int]map[objectpath]dbus.Variant{
	// 0: This will be used to store the VALUE variant
	0: map[objectpath]dbus.Variant{},
	// 1: This will be used to store the STRING variant
	1: map[objectpath]dbus.Variant{},
}

func (f objectpath) GetValue() (dbus.Variant, *dbus.Error) {
	log.Debug("GetValue() called for ", f)
	log.Debug("...returning ", victronValues[0][f])
	return victronValues[0][f], nil
}
func (f objectpath) GetText() (string, *dbus.Error) {
	log.Debug("GetText() called for ", f)
	log.Debug("...returning ", victronValues[1][f])
	// Why does this end up ""SOMEVAL"" ... trim it I guess
	return strings.Trim(victronValues[1][f].String(), "\""), nil
}

func publishInitialValues(conn *dbus.Conn) {

	initDbusVariants()

	if conn == nil {
		return
	}

	// Some of the victron stuff requires it be called grid.cgwacs... using the only known valid value (from the simulator)
	// This can _probably_ be changed as long as it matches com.victronenergy.grid.cgwacs_*
	reply, err := conn.RequestName("com.victronenergy.grid.cgwacs_vzTWO",
		dbus.NameFlagDoNotQueue)
	if err != nil {
		log.Panic("Something went horribly wrong in the dbus connection")
		panic(err)
	}

	if reply != dbus.RequestNameReplyPrimaryOwner {
		log.Panic("name cgwacs_* already taken on dbus.")
		os.Exit(1)
	}

	for i, s := range basicPaths {
		log.Debug("Registering dbus basic path #", i, ": ", s)
		conn.Export(objectpath(s), s, "com.victronenergy.BusItem")
		conn.Export(introspect.Introspectable(intro), s, "org.freedesktop.DBus.Introspectable")
	}

	for i, s := range updatingPaths {
		log.Debug("Registering dbus update path #", i, ": ", s)
		conn.Export(objectpath(s), s, "com.victronenergy.BusItem")
		conn.Export(introspect.Introspectable(intro), s, "org.freedesktop.DBus.Introspectable")
	}
}

func initDbusVariants() {
	// Need to implement following paths:
	// https://github.com/victronenergy/venus/wiki/dbus#grid-meter
	// also in system.py
	victronValues[0]["/Connected"] = dbus.MakeVariant(1)
	victronValues[1]["/Connected"] = dbus.MakeVariant("1")

	victronValues[0]["/CustomName"] = dbus.MakeVariant("Grid meter")
	victronValues[1]["/CustomName"] = dbus.MakeVariant("Grid meter")

	victronValues[0]["/DeviceInstance"] = dbus.MakeVariant(30)
	victronValues[1]["/DeviceInstance"] = dbus.MakeVariant("30")

	// also in system.py
	victronValues[0]["/DeviceType"] = dbus.MakeVariant(71)
	victronValues[1]["/DeviceType"] = dbus.MakeVariant("71")

	victronValues[0]["/ErrorCode"] = dbus.MakeVariantWithSignature(0, dbus.SignatureOf(123))
	victronValues[1]["/ErrorCode"] = dbus.MakeVariant("0")

	victronValues[0]["/FirmwareVersion"] = dbus.MakeVariant(2)
	victronValues[1]["/FirmwareVersion"] = dbus.MakeVariant("2")

	// also in system.py
	victronValues[0]["/Mgmt/Connection"] = dbus.MakeVariant("/dev/ttyUSB2")
	victronValues[1]["/Mgmt/Connection"] = dbus.MakeVariant("/dev/ttyUSB2")

	victronValues[0]["/Mgmt/ProcessName"] = dbus.MakeVariant("/opt/color-control/dbus-cgwacs/dbus-cgwacs")
	victronValues[1]["/Mgmt/ProcessName"] = dbus.MakeVariant("/opt/color-control/dbus-cgwacs/dbus-cgwacs")

	victronValues[0]["/Mgmt/ProcessVersion"] = dbus.MakeVariant("1.8.0")
	victronValues[1]["/Mgmt/ProcessVersion"] = dbus.MakeVariant("1.8.0")

	victronValues[0]["/Position"] = dbus.MakeVariantWithSignature(0, dbus.SignatureOf(123))
	victronValues[1]["/Position"] = dbus.MakeVariant("0")

	// also in system.py
	victronValues[0]["/ProductId"] = dbus.MakeVariant(45058)
	victronValues[1]["/ProductId"] = dbus.MakeVariant("45058")

	// also in system.py
	victronValues[0]["/ProductName"] = dbus.MakeVariant("Grid meter")
	victronValues[1]["/ProductName"] = dbus.MakeVariant("Grid meter")

	victronValues[0]["/Serial"] = dbus.MakeVariant("BP98305081235")
	victronValues[1]["/Serial"] = dbus.MakeVariant("BP98305081235")

	// Provide some initial values... note that the values must be a valid format otherwise dbus_systemcalc.py exits like this:
	// @400000005ecc11bf3782b374   File "/opt/victronenergy/dbus-systemcalc-py/dbus_systemcalc.py", line 386, in _handletimertick
	// @400000005ecc11bf37aa251c     self._updatevalues()
	// @400000005ecc11bf380e74cc   File "/opt/victronenergy/dbus-systemcalc-py/dbus_systemcalc.py", line 678, in _updatevalues
	// @400000005ecc11bf383ab4ec     c = _safeadd(c, p, pvpower)
	// @400000005ecc11bf386c9674   File "/opt/victronenergy/dbus-systemcalc-py/sc_utils.py", line 13, in safeadd
	// @400000005ecc11bf387b28ec     return sum(values) if values else None
	// @400000005ecc11bf38b2bb7c TypeError: unsupported operand type(s) for +: 'int' and 'unicode'
	//
	victronValues[0]["/Ac/L1/Power"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L1/Power"] = dbus.MakeVariant("0 W")
	victronValues[0]["/Ac/L2/Power"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L2/Power"] = dbus.MakeVariant("0 W")
	victronValues[0]["/Ac/L3/Power"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L3/Power"] = dbus.MakeVariant("0 W")

	victronValues[0]["/Ac/L1/Voltage"] = dbus.MakeVariant(230)
	victronValues[1]["/Ac/L1/Voltage"] = dbus.MakeVariant("230 V")
	victronValues[0]["/Ac/L2/Voltage"] = dbus.MakeVariant(230)
	victronValues[1]["/Ac/L2/Voltage"] = dbus.MakeVariant("230 V")
	victronValues[0]["/Ac/L3/Voltage"] = dbus.MakeVariant(230)
	victronValues[1]["/Ac/L3/Voltage"] = dbus.MakeVariant("230 V")

	victronValues[0]["/Ac/L1/Current"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L1/Current"] = dbus.MakeVariant("0 A")
	victronValues[0]["/Ac/L2/Current"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L2/Current"] = dbus.MakeVariant("0 A")
	victronValues[0]["/Ac/L3/Current"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L3/Current"] = dbus.MakeVariant("0 A")

	victronValues[0]["/Ac/L1/Energy/Forward"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L1/Energy/Forward"] = dbus.MakeVariant("0 kWh")
	victronValues[0]["/Ac/L2/Energy/Forward"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L2/Energy/Forward"] = dbus.MakeVariant("0 kWh")
	victronValues[0]["/Ac/L3/Energy/Forward"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L3/Energy/Forward"] = dbus.MakeVariant("0 kWh")

	victronValues[0]["/Ac/L1/Energy/Reverse"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L1/Energy/Reverse"] = dbus.MakeVariant("0 kWh")
	victronValues[0]["/Ac/L2/Energy/Reverse"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L2/Energy/Reverse"] = dbus.MakeVariant("0 kWh")
	victronValues[0]["/Ac/L3/Energy/Reverse"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L3/Energy/Reverse"] = dbus.MakeVariant("0 kWh")

	victronValues[0]["/Ac/Frequency"] = dbus.MakeVariant(50.0)
	victronValues[1]["/Ac/Frequency"] = dbus.MakeVariant("50 Hz")

	basicPaths = []dbus.ObjectPath{
		"/Connected",
		"/CustomName",
		"/DeviceInstance",
		"/DeviceType",
		"/ErrorCode",
		"/FirmwareVersion",
		"/Mgmt/Connection",
		"/Mgmt/ProcessName",
		"/Mgmt/ProcessVersion",
		"/Position",
		"/ProductId",
		"/ProductName",
		"/Serial",
	}

	updatingPaths = []dbus.ObjectPath{
		"/Ac/L1/Power",
		"/Ac/L2/Power",
		"/Ac/L3/Power",
		"/Ac/L1/Voltage",
		"/Ac/L2/Voltage",
		"/Ac/L3/Voltage",
		"/Ac/L1/Current",
		"/Ac/L2/Current",
		"/Ac/L3/Current",
		"/Ac/L1/Energy/Forward",
		"/Ac/L2/Energy/Forward",
		"/Ac/L3/Energy/Forward",
		"/Ac/L1/Energy/Reverse",
		"/Ac/L2/Energy/Reverse",
		"/Ac/L3/Energy/Reverse",
		"/Ac/Frequency",
	}

}

func updateVariantFromData(conn *dbus.Conn, data SmartMeterData) {

	defaultVoltage := 230
	splittedPower := float64(data.ActualPower / 3)   // split to 3 lines
	splittedEnergyGridIn := data.GridIn / 3 / 1000   //Wh to kWh and split to 3 lines
	splittedEnergyGridOut := data.GridOut / 3 / 1000 //Wh to kWh and split to 3 lines

	updateVariant(conn, splittedPower, "W", "/Ac/L1/Power")
	updateVariant(conn, splittedPower, "W", "/Ac/L2/Power")
	updateVariant(conn, splittedPower, "W", "/Ac/L3/Power")

	updateVariant(conn, data.GridIn/1000, "kWh", "/Ac/Energy/Forward")  //Wh to kWh
	updateVariant(conn, data.GridOut/1000, "kWh", "/Ac/Energy/Reverse") //Wh to kWh

	updateVariant(conn, float64(data.ActualPower), "W", "/Ac/Power")

	updateVariant(conn, splittedPower/float64(defaultVoltage), "A", "/Ac/L1/Current")
	updateVariant(conn, splittedPower/float64(defaultVoltage), "A", "/Ac/L2/Current")
	updateVariant(conn, splittedPower/float64(defaultVoltage), "A", "/Ac/L3/Current")

	updateVariant(conn, float64(defaultVoltage), "V", "/Ac/L1/Voltage")
	updateVariant(conn, float64(defaultVoltage), "V", "/Ac/L2/Voltage")
	updateVariant(conn, float64(defaultVoltage), "V", "/Ac/L3/Voltage")

	updateVariant(conn, splittedEnergyGridIn, "kWh", "/Ac/L1/Energy/Forward")
	updateVariant(conn, splittedEnergyGridIn, "kWh", "/Ac/L2/Energy/Forward")
	updateVariant(conn, splittedEnergyGridIn, "kWh", "/Ac/L3/Energy/Forward")

	updateVariant(conn, splittedEnergyGridOut, "kWh", "/Ac/L1/Energy/Reverse")
	updateVariant(conn, splittedEnergyGridOut, "kWh", "/Ac/L2/Energy/Reverse")
	updateVariant(conn, splittedEnergyGridOut, "kWh", "/Ac/L3/Energy/Reverse")

	updateVariant(conn, 50.0, "Hz", "/Ac/Frequency")

}

/* Write dbus Values to Victron handler */
func updateVariant(conn *dbus.Conn, value float64, unit string, path string) {

	if conn == nil {
		return
	}

	emit := make(map[string]dbus.Variant)
	emit["Text"] = dbus.MakeVariant(fmt.Sprintf("%.2f", value) + unit)
	emit["Value"] = dbus.MakeVariant(float64(value))
	victronValues[0][objectpath(path)] = emit["Value"]
	victronValues[1][objectpath(path)] = emit["Text"]
	conn.Emit(dbus.ObjectPath(path), "com.victronenergy.BusItem.PropertiesChanged", emit)
}
