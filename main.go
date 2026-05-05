package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	log "github.com/sirupsen/logrus"
	//"vz-mqtt-dbus-gateway/sml/Message"
)

var Version = "dev"

func setupLogging(logLevel string) {
	ll, err := log.ParseLevel(logLevel)
	if err != nil {
		ll = log.InfoLevel
	}

	log.SetLevel(ll)
}

func main() {
	logLevelEnv, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		logLevelEnv = "info"
	}

	logLevel := flag.String("log-level", logLevelEnv, "Log level (debug, info, warn, error, fatal, panic)")
	mqttServer := flag.String("server", "192.168.178.3:1883", "IP:Port")
	mqttTopic := flag.String("topic", "/smartmeter1/power", "Topic to subscribe to")
	mqttQos := flag.Int("qos", 0, "The QoS to subscribe to messages at")
	mqttClientId := flag.String("clientid", "vz-mqtt-dbus-bridge", "A clientid for the connection")
	username := flag.String("username", "", "A username to authenticate to the MQTT server")
	password := flag.String("password", "", "Password to match username")
	publishStatics := flag.Bool("publish_statics", false, "true/false")
	forceExit := flag.Bool("force-exit", false, "Call os.Exit(1) when watchdog is triggered")
	flag.Parse()

	setupLogging(*logLevel)

	messages := make(chan SmartMeterData)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	conn, err := dbus.SystemBus()

	if err != nil {
		log.Fatalf("Could not connect to Systembus: %v", err)
	}
	defer conn.Close()

	log.Info("DBUS: connected to Systembus")

	forceExitVal := *forceExit
	watchdog := CreateWatchdog(time.Second*10, func() {
		log.Error("Watchdog: triggered, marking data as invalid")
		invalidateData(conn)
		if forceExitVal {
			os.Exit(1)
		}
	})

	initDbus(conn, publishStatics)
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
			startMqttGateway(messages, *mqttServer, *mqttTopic, *mqttQos, *mqttClientId, *username, *password)
			log.Warn("Gateway: MQTT gateway stopped, retrying in 5 seconds")
			time.Sleep(5 * time.Second)
		}
	}()

	sig := <-signalChan
	log.Infof("Gateway: received signal %v, shutting down", sig)
	invalidateData(conn)
	conn.Close()
}
