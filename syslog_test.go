package main

import (
	"net"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
)

func TestFormatRFC5424(t *testing.T) {
	ts := time.Date(2026, 10, 9, 23, 55, 29, 123456789, time.FixedZone("CEST", 2*3600))

	cases := []struct {
		entry    *log.Entry
		expected string
	}{
		{
			&log.Entry{Time: ts, Level: log.InfoLevel, Message: "MQTT: Connected to 192.168.178.3:1883\n"},
			"<30>1 2026-10-09T21:55:29.123456Z venus vz-mqtt-dbus-gateway 42 - - MQTT: Connected to 192.168.178.3:1883",
		},
		{
			&log.Entry{Time: ts, Level: log.ErrorLevel, Message: "failed", Data: log.Fields{"b": 2, "a": "x"}},
			"<27>1 2026-10-09T21:55:29.123456Z venus vz-mqtt-dbus-gateway 42 - - failed a=x b=2",
		},
		{
			&log.Entry{Time: ts, Level: log.FatalLevel, Message: "fatal"},
			"<26>1 2026-10-09T21:55:29.123456Z venus vz-mqtt-dbus-gateway 42 - - fatal",
		},
	}

	for _, c := range cases {
		if got := formatRFC5424(c.entry, "venus", 42); got != c.expected {
			t.Errorf("got      %q\nexpected %q", got, c.expected)
		}
	}
}

func TestSyslogHookSendsUDP(t *testing.T) {
	server, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	logger := log.New()
	conn, err := net.Dial("udp", server.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	logger.AddHook(&rfc5424Hook{conn: conn, hostname: "venus", pid: 42})
	logger.Info("first")
	logger.Info("second")

	buf := make([]byte, 1024)
	for _, expected := range []string{"first", "second"} {
		_ = server.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _, err := server.ReadFrom(buf)
		if err != nil {
			t.Fatal(err)
		}
		if msg := string(buf[:n]); !strings.HasPrefix(msg, "<30>1 ") || !strings.HasSuffix(msg, " - - "+expected) {
			t.Errorf("unexpected syslog message %q", msg)
		}
	}
}
