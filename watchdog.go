package main

import "time"

type Watchdog struct {
	interval time.Duration
	timer    *time.Timer
}

func CreateWatchdog(interval time.Duration, callback func()) *Watchdog {
	w := Watchdog{
		interval: interval,
		timer:    time.AfterFunc(interval, callback),
	}
	return &w
}

func (w *Watchdog) ResetWatchdog() {
	w.timer.Stop()
	w.timer.Reset(w.interval)
}
