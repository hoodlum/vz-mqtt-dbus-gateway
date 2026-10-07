//go:build !windows
// +build !windows

package main

import (
	"log/syslog"

	log "github.com/sirupsen/logrus"
	lsyslog "github.com/sirupsen/logrus/hooks/syslog"
)

func setupSyslog(addr string) error {
	hook, err := lsyslog.NewSyslogHook("udp", addr, syslog.LOG_INFO|syslog.LOG_DAEMON, "vz-mqtt-dbus-gateway")
	if err != nil {
		return err
	}
	log.AddHook(hook)
	return nil
}
