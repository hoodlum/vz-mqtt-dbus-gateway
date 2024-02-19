package main

import (
	"context"
	"encoding/json"
	"flag"
	"github.com/eclipse/paho.golang/paho"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// VZ message structure
type SmartMeterData struct {
	TimeStamp UnixTime `json:"ts"`
	//SensorId    byte     `json:"sensorId"`
	GridIn float64 `json:"energy1_8_1"`
	//Energy1_8_2 float64  `json:"energy1_8_2"`
	GridOut     float64 `json:"energy2_8_0"`
	ActualPower int32   `json:"power16_7_0"`
}

func startMqttGateway(messages chan SmartMeterData) {

	mqttServer := flag.String("server", "192.168.178.3:1883", "IP:Port")
	mqttTopic := flag.String("topic", "/smartmeter1/power", "Topic to subscribe to")
	mqttQos := flag.Int("qos", 0, "The QoS to subscribe to messages at")
	mqttClientId := flag.String("clientid", "vz-mqtt-dbus-bridge", "A clientid for the connection")
	username := flag.String("username", "", "A username to authenticate to the MQTT server")
	password := flag.String("password", "", "Password to match username")
	flag.Parse()

	//logger := log.New(os.Stdout, "SUB: ", log.LstdFlags)

	msgChan := make(chan *paho.Publish)

	conn, err := net.Dial("tcp", *mqttServer)
	if err != nil {
		log.Fatalf("Failed to connect to %s: %s", *mqttServer, err)
	}

	c := paho.NewClient(paho.ClientConfig{
		Router: paho.NewSingleHandlerRouter(func(m *paho.Publish) {
			msgChan <- m
		}),
		Conn: conn,
	})

	//c.SetDebugLogger(logger)
	c.SetErrorLogger(log.StandardLogger())

	cp := &paho.Connect{
		KeepAlive:  30,
		ClientID:   *mqttClientId,
		CleanStart: true,
		//Username:   *username,
		//Password:   []byte(*password),
	}

	if *username != "" {
		cp.UsernameFlag = true
	}

	if *password != "" {
		cp.PasswordFlag = true
	}

	ca, err := c.Connect(context.Background(), cp)
	if err != nil {
		log.Fatalln(err)
	}

	if ca.ReasonCode != 0 {
		log.Fatalf("Failed to connect to %s : %d - %s", *mqttServer, ca.ReasonCode, ca.Properties.ReasonString)
	}

	log.Infof("MQTT: Connected to %s\n", *mqttServer)

	ic := make(chan os.Signal, 1)
	signal.Notify(ic, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ic
		log.Infof("signal received, exiting")
		if c != nil {
			d := &paho.Disconnect{ReasonCode: 0}
			c.Disconnect(d)
		}
		os.Exit(0)
	}()

	sa, err := c.Subscribe(context.Background(), &paho.Subscribe{
		Subscriptions: map[string]paho.SubscribeOptions{
			*mqttTopic: {QoS: byte(*mqttQos)},
		},
	})
	if err != nil {
		log.Fatalln(err)
	}

	if sa.Reasons[0] != byte(*mqttQos) {
		log.Fatalf("MQTT: Failed to subscribe to %s : %d", *mqttTopic, sa.Reasons[0])
	}

	log.Infof("MQTT: Subscribed to %s, starting Dispatcher", *mqttTopic)

	// Map from MQTT and push to DBUS
	for m := range msgChan {

		message := string(m.Payload)
		log.Debugf("Received message:", message)

		var data SmartMeterData

		err := json.Unmarshal([]byte(message), &data)
		if err == nil {
			log.Debugf("Received json:", data)
			messages <- data
		}
	}

	log.Fatal("MQTT: Passed Dispatcher loop, how did get here?")

}

type UnixTime struct {
	time.Time
}

func (u *UnixTime) UnmarshalJSON(b []byte) error {
	var timestamp int64
	err := json.Unmarshal(b, &timestamp)
	if err != nil {
		return err
	}
	u.Time = time.Unix(timestamp/1000, timestamp%1000)
	return nil
}
