package main

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
)

const syslogAppName = "vz-mqtt-dbus-gateway"

// facility daemon (3), see RFC 5424 section 6.2.1
const syslogFacilityDaemon = 3

// rfc5424Hook forwards log entries as RFC 5424 syslog messages via UDP. Go's
// log/syslog only writes timestamps with second precision, so a syslog server
// cannot keep the order of several entries within the same second.
type rfc5424Hook struct {
	conn     net.Conn
	hostname string
	pid      int
}

func setupSyslog(addr string) error {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return err
	}

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "-"
	}

	log.AddHook(&rfc5424Hook{conn: conn, hostname: hostname, pid: os.Getpid()})
	return nil
}

func (h *rfc5424Hook) Levels() []log.Level {
	return log.AllLevels
}

func (h *rfc5424Hook) Fire(entry *log.Entry) error {
	_, err := h.conn.Write([]byte(formatRFC5424(entry, h.hostname, h.pid)))
	return err
}

// syslogSeverity uses the same mapping as logrus/hooks/syslog.
func syslogSeverity(level log.Level) int {
	switch level {
	case log.PanicLevel, log.FatalLevel:
		return 2 // crit
	case log.ErrorLevel:
		return 3 // err
	case log.WarnLevel:
		return 4 // warning
	case log.InfoLevel:
		return 6 // info
	default:
		return 7 // debug
	}
}

// formatRFC5424 renders <PRI>1 TIMESTAMP HOSTNAME APP-NAME PROCID MSGID SD MSG
// with a microsecond UTC timestamp.
func formatRFC5424(entry *log.Entry, hostname string, pid int) string {
	msg := strings.TrimSpace(entry.Message)

	keys := make([]string, 0, len(entry.Data))
	for k := range entry.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		msg += fmt.Sprintf(" %s=%v", k, entry.Data[k])
	}

	return fmt.Sprintf("<%d>1 %s %s %s %d - - %s",
		syslogFacilityDaemon*8+syslogSeverity(entry.Level),
		entry.Time.UTC().Format("2006-01-02T15:04:05.000000Z"),
		hostname, syslogAppName, pid, msg)
}
