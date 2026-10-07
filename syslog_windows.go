//go:build windows
// +build windows

package main

import "errors"

func setupSyslog(addr string) error {
	return errors.New("syslog is not supported on windows")
}
