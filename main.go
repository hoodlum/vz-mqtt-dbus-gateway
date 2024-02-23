package main

import (
	"github.com/godbus/dbus/v5"
	log "github.com/sirupsen/logrus"
	"os"
	"time"
	//"vz-mqtt-dbus-gateway/sml/Message"
)

func init() {
	lvl, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		lvl = "info"
	}

	ll, err := log.ParseLevel(lvl)
	if err != nil {
		ll = log.DebugLevel
	}

	log.SetLevel(ll)
}

func main() {

	messages := make(chan SmartMeterData)
	signal := make(chan bool)

	var conn, err = dbus.SystemBus()

	if err != nil {
		log.Info("Could not connect to Systembus")
	}

	log.Info("DBUS: connected to Systembus")

	watchdog := CreateWatchdog(time.Second*10, func() {
		//fmt.Println("Watchdog triggered, handle situation")
		log.Error("Watchdog: triggered, kill process to allow restart by venus-os")
		signal <- true
	})
	//watchdog.ResetWatchdog()

	initDbus(conn)
	log.Info("DBUS: Registered as a meter")

	//Dispatcher
	go func() {
		log.Info("Gateway: Dispatcher started")
		//initNeeded := true
		for m := range messages {
			//if initNeeded {

			//				log.Info("DBUS: Registered as a meter")
			//				initNeeded = false
			//			}
			watchdog.ResetWatchdog()
			pushSmartmeterData(conn, m)
		}
		log.Info("Gateway: Finish Handler")
	}()

	startMqttGateway(messages)

	for _ = range signal {
		log.Info("Gateway: got signal from watchdog to shutdown")
		break
	}

	defer conn.Close()

	//	log.Info("Successfully connected to dbus and registered as a meter... Commencing reading of the SDM630 meter")

	// This is a forever loop^^
	//panic("Error: We terminated.... how did we ever get here?")
}
