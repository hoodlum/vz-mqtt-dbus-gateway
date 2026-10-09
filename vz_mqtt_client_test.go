package main

import (
	"net"
	"testing"
	"time"

	"github.com/eclipse/paho.golang/packets"
)

const testTopic = "/smartmeter1/power"

// fakeBroker accepts the connect and the subscription, publishes one
// smart meter message and returns the received CONNECT packet.
func fakeBroker(t *testing.T, conn net.Conn) *packets.Connect {
	t.Helper()

	cp, err := packets.ReadPacket(conn)
	if err != nil || cp.Type != packets.CONNECT {
		t.Fatalf("expected CONNECT, got %v (%v)", cp, err)
	}
	connack := packets.NewControlPacket(packets.CONNACK)
	connack.Content.(*packets.Connack).Properties = &packets.Properties{}
	if _, err := connack.WriteTo(conn); err != nil {
		t.Fatal(err)
	}

	sp, err := packets.ReadPacket(conn)
	if err != nil || sp.Type != packets.SUBSCRIBE {
		t.Fatalf("expected SUBSCRIBE, got %v (%v)", sp, err)
	}
	suback := packets.NewControlPacket(packets.SUBACK)
	suback.Content.(*packets.Suback).PacketID = sp.Content.(*packets.Subscribe).PacketID
	suback.Content.(*packets.Suback).Reasons = []byte{0}
	suback.Content.(*packets.Suback).Properties = &packets.Properties{}
	if _, err := suback.WriteTo(conn); err != nil {
		t.Fatal(err)
	}

	pub := packets.NewControlPacket(packets.PUBLISH)
	pub.Content.(*packets.Publish).Topic = testTopic
	pub.Content.(*packets.Publish).Payload = []byte(`{"ts":1791584352000,"energy1_8_1":1.5,"energy2_8_0":2.5,"power16_7_0":-420}`)
	pub.Content.(*packets.Publish).Properties = &packets.Properties{}
	if _, err := pub.WriteTo(conn); err != nil {
		t.Fatal(err)
	}

	return cp.Content.(*packets.Connect)
}

// startSession runs runMqttSession against a fake broker, waits for the
// published message and returns the broker side and a channel that is
// closed when the session returns.
func startSession(t *testing.T, username, password string) (net.Conn, *packets.Connect, chan struct{}) {
	t.Helper()

	client, server := net.Pipe()
	t.Cleanup(func() { client.Close(); server.Close() })

	messages := make(chan SmartMeterData)
	returned := make(chan struct{})
	go func() {
		runMqttSession(client, messages, "test", testTopic, 0, "test-client", username, password)
		close(returned)
	}()

	connect := fakeBroker(t, server)

	select {
	case m := <-messages:
		if m.ActualPower != -420 || m.GridIn != 1.5 || m.GridOut != 2.5 {
			t.Errorf("unexpected message %+v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no message forwarded")
	}

	return server, connect, returned
}

func waitReturned(t *testing.T, returned chan struct{}) {
	t.Helper()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("runMqttSession did not return after the connection was lost")
	}
}

func TestMqttSessionReturnsOnConnectionLoss(t *testing.T) {
	server, _, returned := startSession(t, "", "")
	server.Close()
	waitReturned(t, returned)
}

func TestMqttSessionReturnsOnServerDisconnect(t *testing.T) {
	server, _, returned := startSession(t, "", "")
	disconnect := packets.NewControlPacket(packets.DISCONNECT)
	disconnect.Content.(*packets.Disconnect).Properties = &packets.Properties{}
	if _, err := disconnect.WriteTo(server); err != nil {
		t.Fatal(err)
	}
	waitReturned(t, returned)
}

func TestMqttSessionSendsCredentials(t *testing.T) {
	cases := []struct {
		username, password string
	}{
		{"", ""},
		{"meter", ""},
		{"meter", "secret"},
	}

	for _, c := range cases {
		server, connect, returned := startSession(t, c.username, c.password)
		if connect.UsernameFlag != (c.username != "") || connect.Username != c.username {
			t.Errorf("username %q: got flag %v, value %q", c.username, connect.UsernameFlag, connect.Username)
		}
		if connect.PasswordFlag != (c.password != "") || string(connect.Password) != c.password {
			t.Errorf("password %q: got flag %v, value %q", c.password, connect.PasswordFlag, connect.Password)
		}
		server.Close()
		waitReturned(t, returned)
	}
}
