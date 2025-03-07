package probes

import (
	"errors"
	"time"
)

type probeTask struct {
	timeout  time.Duration
	interval time.Duration
	nextRun  time.Time
	locked   bool
	label    string
}

func (this *probeTask) Label() string {
	return this.label
}

func (this *probeTask) Interval() time.Duration {
	return this.interval
}

func (this *probeTask) Ready() bool {
	return !this.locked && time.Now().After(this.nextRun)
}

func (this *probeTask) Lock() error {

	if this.locked {
		return errors.New("task already locked")
	}

	this.locked = true

	return nil
}

func (this *probeTask) Unlock() error {

	if !this.locked {
		return errors.New("task not locked")
	}

	this.nextRun = time.Now().Add(this.interval)
	this.locked = false
	return nil
}
