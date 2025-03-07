package probes

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/maddsua/pulse/config"
	socks "github.com/maddsua/pulse/proxy"
	"golang.org/x/net/proxy"
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

func loadProxy(proxyKey string, proxies config.ProxyConfigMap) (proxy.ContextDialer, error) {

	if len(proxies) == 0 {
		return nil, errors.New("no proxies defined in the config")
	}

	proxyCfg, has := proxies[proxyKey]
	if !has || proxyCfg == nil {
		return nil, errors.New("proxy tag not found")
	}

	proxyUrl, err := url.Parse(proxyCfg.Url)
	if err != nil {
		return nil, fmt.Errorf("proxy url invalid: %s", err.Error())
	}

	dialer, err := socks.NewSocksProxyDialer(proxyUrl.Host, proxyUrl.User)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy dialer: %s", err.Error())
	}

	return dialer, nil
}
