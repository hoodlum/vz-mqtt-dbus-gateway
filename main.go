package main

import (
	"github.com/godbus/dbus/v5"
	log "github.com/sirupsen/logrus"
	"os"
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

	log.Info("connected to systembus: ")

	go func() {
		for m := range messages {
			updateVariantFromData(conn, m)
		}
		log.Info("Finish Handler")
	}()

	publishInitialValues(conn)

	startMqttGateway(messages)

	defer conn.Close()

	log.Info("Successfully connected to dbus and registered as a meter... Commencing reading of the SDM630 meter")

	// This is a forever loop^^
	panic("Error: We terminated.... how did we ever get here?")
}

/*
func decode(byteMessage []byte) {
	message, _, err := Message.New(0, byteMessage)

	if err != nil || message == nil {
		log.Println("ERROR - ")
	}

	log.Println("id: ", message.MessageBody.TransactionId)
	//result := make([]byte, 4*128)
	//buff := bytes.NewBuffer(result)
	//for _, b := range message {
	//	fmt.Fprintf(buff, "%02x ", b)
	//}
	//log.Println(buff.String())

	//log.Println(string(buf[:n]))
}
*/
