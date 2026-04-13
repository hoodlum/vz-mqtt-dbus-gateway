package main

import (
	"context"
	"github.com/godbus/dbus/v5"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
	"time"
	//"vz-mqtt-dbus-gateway/sml/Message"
)

var Version = "dev"

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
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	conn, err := dbus.SystemBus()

	if err != nil {
		log.Fatalf("Could not connect to Systembus: %v", err)
	}
	defer conn.Close()

	log.Info("DBUS: connected to Systembus")

	watchdog := CreateWatchdog(time.Second*10, func() {
		log.Error("Watchdog: triggered, marking data as invalid and killing process")
		invalidateData(conn)
		os.Exit(1)
	})

	initDbus(conn)
	log.Info("DBUS: Registered as a meter")

	//Dispatcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		log.Info("Gateway: Dispatcher started")
		for {
			select {
			case m, ok := <-messages:
				if !ok {
					log.Info("Gateway: message channel closed")
					return
				}
				watchdog.ResetWatchdog()
				pushSmartmeterData(conn, m)
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		for {
			log.Info("Gateway: Starting MQTT gateway")
			startMqttGateway(messages)
			log.Warn("Gateway: MQTT gateway stopped, retrying in 5 seconds")
			time.Sleep(5 * time.Second)
		}
	}()

	sig := <-signalChan
	log.Infof("Gateway: received signal %v, shutting down", sig)
	invalidateData(conn)
}
