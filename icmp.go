package pulse

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/tatsushid/go-fastping"
)

type IcmpProbe struct {
	IcmpProbeOptions

	Label  string
	Writer StorageWriter

	locked   atomic.Bool
	nextExec time.Time
}

type IcmpProbeOptions struct {
	Interval time.Duration `yaml:"interval" json:"interval"`
	Timeout  time.Duration `yaml:"timeout" json:"timeout"`
	Host     string        `yaml:"host" json:"host"`
	Retries  int           `yaml:"retries" json:"retries"`
}

func (this *IcmpProbe) ID() string {
	return this.Label
}

func (this *IcmpProbe) Type() string {
	return "icmp"
}

func (this *IcmpProbe) validateConfig() error {

	switch {
	case this.Label == "":
		return errors.New("label is empty")
	case this.Host == "":
		return errors.New("empty host")
	}

	if this.Interval <= time.Second {
		this.Interval = time.Minute
	}

	return nil
}

func (this *IcmpProbe) Ready() (bool, error) {

	if err := this.validateConfig(); err != nil {
		return false, err
	}

	if this.Writer == nil {
		return false, errors.New("writer is nil")
	}

	//	check locks
	if this.locked.Load() {
		return false, nil
	}

	if this.nextExec.IsZero() || this.nextExec.Before(time.Now()) {
		this.nextExec = time.Now().Add(this.Interval)
		return true, nil
	}

	return false, nil
}

func (this *IcmpProbe) Exec(ctx context.Context) error {

	if _, err := this.Ready(); err != nil {
		return err
	}

	if !this.locked.CompareAndSwap(false, true) {
		return errors.New("task locked")
	}
	defer this.locked.Store(false)

	timeout := 10 * time.Second
	if this.IcmpProbeOptions.Timeout > 0 {
		timeout = this.IcmpProbeOptions.Timeout
	}

	type pingStatus struct {
		ResolvedAddr net.IP
		Online       bool
		Latency      time.Duration
	}

	var fetchStatus = func(ctx context.Context) (*pingStatus, error) {

		addr, err := net.ResolveIPAddr("ip", this.Host)
		if err != nil {
			return &pingStatus{}, nil
		}

		pinger := fastping.NewPinger()
		pinger.MaxRTT = timeout
		pinger.AddIPAddr(addr)

		statusCh := make(chan pingStatus)
		errorCh := make(chan error)

		pinger.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
			statusCh <- pingStatus{ResolvedAddr: addr.IP, Online: true, Latency: rtt}
		}

		go func() {
			if err := pinger.Run(); err != nil {
				errorCh <- err
				return
			}
			statusCh <- pingStatus{ResolvedAddr: addr.IP}
		}()

		select {

		case status := <-statusCh:
			return &status, nil

		case err := <-errorCh:
			return nil, err

		case <-ctx.Done():
			return &pingStatus{ResolvedAddr: addr.IP}, nil
		}
	}

	pingCtx, cancelPing := context.WithTimeout(ctx, timeout)
	defer cancelPing()

	started := time.Now()

	status, err := fetchStatus(pingCtx)
	if err != nil {
		return err
	}

	if status.ResolvedAddr != nil && !status.Online && this.IcmpProbeOptions.Retries > 0 {
		for n := 0; n < this.IcmpProbeOptions.Retries && !status.Online && pingCtx.Err() == nil; n++ {
			if status, err = fetchStatus(pingCtx); err != nil {
				return err
			}
		}
	}

	entry := UptimeEntry{
		Label:        this.Label,
		Timestamp:    time.Now(),
		ProbeType:    this.Type(),
		ProbeElapsed: time.Since(started),
		Up:           status.Online,
	}

	if status.ResolvedAddr != nil {
		host := status.ResolvedAddr.String()
		entry.Host = &host
	}

	if status.Online {
		entry.Latency = &status.Latency
	}

	if err := this.Writer.WriteUptime(ctx, entry); err != nil {
		return fmt.Errorf("storage.WriteUptime: %v", err)
	}

	return nil
}
