package main

import (
	"fmt"
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

	var conn, err = dbus.SystemBus()

	if err != nil {
		log.Info("Could not connect to Systembus")
	}

	log.Info("DBUS: connected to Systembus")

	watchdog := CreateWatchdog(time.Second*10, func() {
		fmt.Println("Watchdog triggered, handle situation")
		log.Fatal("Grace period exceeded, kill process to allow restart by venus-os")
	})

	//Dispatcher
	go func() {
		log.Info("Gateway: Dispatcher started")
		initNeeded := true
		for m := range messages {
			if initNeeded {
				publishInitialValues(conn)
				log.Info("DBUS: Registered as a meter")
				initNeeded = false
			}
			watchdog.ResetWatchdog()
			updateVariantFromData(conn, m)
		}
		log.Info("Finish Handler")
	}()

	startMqttGateway(messages)

	defer conn.Close()

	//	log.Info("Successfully connected to dbus and registered as a meter... Commencing reading of the SDM630 meter")

	// This is a forever loop^^
	panic("Error: We terminated.... how did we ever get here?")
}
