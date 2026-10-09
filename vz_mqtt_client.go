package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/paho"
	log "github.com/sirupsen/logrus"
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

// newConnectPacket only sends credentials that are set, an anonymous broker
// may reject a connect with an empty username.
func newConnectPacket(clientId, username, password string) *paho.Connect {
	cp := &paho.Connect{
		KeepAlive:  30,
		ClientID:   clientId,
		CleanStart: true,
	}

	if username != "" {
		cp.UsernameFlag = true
		cp.Username = username
	}
	if password != "" {
		cp.PasswordFlag = true
		cp.Password = []byte(password)
	}

	return cp
}

func startMqttGateway(messages chan SmartMeterData, mqttServer string, mqttTopic string, mqttQos int, mqttClientId string, username string, password string) {

	conn, err := net.DialTimeout("tcp", mqttServer, 5*time.Second)
	if err != nil {
		log.Errorf("Failed to connect to %s: %s", mqttServer, err)
		return
	}
	defer conn.Close()

	runMqttSession(conn, messages, mqttServer, mqttTopic, mqttQos, mqttClientId, username, password)
}

// runMqttSession forwards smart meter messages until the connection is lost,
// so the caller can reconnect.
func runMqttSession(conn net.Conn, messages chan SmartMeterData, mqttServer string, mqttTopic string, mqttQos int, mqttClientId string, username string, password string) {

	//logger := log.New(os.Stdout, "SUB: ", log.LstdFlags)

	msgChan := make(chan *paho.Publish)
	// closed on connection loss, ends the dispatcher so main reconnects
	done := make(chan struct{})
	var doneOnce sync.Once
	connectionLost := func(reason string) {
		doneOnce.Do(func() {
			log.Warnf("MQTT: connection lost: %s", reason)
			close(done)
		})
	}

	c := paho.NewClient(paho.ClientConfig{
		Router: paho.NewSingleHandlerRouter(func(m *paho.Publish) {
			select {
			case msgChan <- m:
			case <-done:
			}
		}),
		Conn: conn,
		OnClientError: func(err error) {
			connectionLost(err.Error())
		},
		OnServerDisconnect: func(d *paho.Disconnect) {
			connectionLost(fmt.Sprintf("server disconnect, reason code %d", d.ReasonCode))
		},
	})

	//c.SetDebugLogger(logger)
	c.SetErrorLogger(log.StandardLogger())

	cp := newConnectPacket(mqttClientId, username, password)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ca, err := c.Connect(ctx, cp)
	if err != nil {
		log.Errorln(err)
		return
	}

	if ca.ReasonCode != 0 {
		reason := ""
		if ca.Properties != nil {
			reason = ca.Properties.ReasonString
		}
		log.Errorf("Failed to connect to %s : %d - %s", mqttServer, ca.ReasonCode, reason)
		return
	}

	log.Infof("MQTT: Connected to %s", mqttServer)

	sa, err := c.Subscribe(context.Background(), &paho.Subscribe{
		Subscriptions: map[string]paho.SubscribeOptions{
			mqttTopic: {QoS: byte(mqttQos)},
		},
	})
	if err != nil {
		log.Errorln(err)
		return
	}

	if sa.Reasons[0] != byte(mqttQos) {
		log.Errorf("MQTT: Failed to subscribe to %s : %d", mqttTopic, sa.Reasons[0])
		return
	}

	log.Infof("MQTT: Subscribed to %s, starting Dispatcher", mqttTopic)

	//Dispatcher
	for {
		select {
		case m := <-msgChan:
			var data SmartMeterData
			if err := json.Unmarshal(m.Payload, &data); err == nil {
				messages <- data
			}
		case <-done:
			return
		}
	}
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
