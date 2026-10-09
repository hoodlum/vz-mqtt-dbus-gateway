package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	log "github.com/sirupsen/logrus"
	//"vz-mqtt-dbus-gateway/sml/Message"
)

func setupLogging(logLevel string) {
	ll, err := log.ParseLevel(logLevel)
	if err != nil {
		ll = log.InfoLevel
	}

	log.SetLevel(ll)
	// microseconds keep the order of entries logged within the same second
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true, TimestampFormat: "2006-01-02T15:04:05.000000Z07:00"})
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
	syslogAddr := flag.String("syslog", "", "Forward logs to a remote syslog server via UDP (host:port)")
	showVersion := flag.Bool("version", false, "Print version information and exit")
	flag.Parse()

	info, _ := debug.ReadBuildInfo()
	resolveVersion(info)
	if *showVersion {
		fmt.Println(versionString())
		return
	}

	setupLogging(*logLevel)

	if *syslogAddr != "" {
		if err := setupSyslog(*syslogAddr); err != nil {
			log.Warnf("Syslog: could not set up forwarding to %s: %v", *syslogAddr, err)
		} else {
			log.Infof("Syslog: forwarding logs to %s", *syslogAddr)
		}
	}

	log.Infof("Gateway: starting %s", versionString())

	messages := make(chan SmartMeterData)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	conn, err := dbus.SystemBus()

	if err != nil {
		log.Fatalf("Could not connect to Systembus: %v", err)
	}
	defer conn.Close()

	log.Info("DBUS: connected to Systembus")

	initDbus(conn, publishStatics)
	log.Info("DBUS: Registered as a meter")

	watchdog := CreateWatchdog(time.Second*10, func() {
		log.Error("Watchdog: triggered, marking data as invalid")
		invalidateData(conn)
		if *forceExit {
			os.Exit(1)
		}
	})
	log.Info("Watchdog: Started")

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

}
