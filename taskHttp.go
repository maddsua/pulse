package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/guregu/null"
	"github.com/maddsua/pulse/storage"
)

func NewHttpTask(label string, opts HttpProbeConfig, proxies ProxyConfigMap) (*httpProbeTask, error) {

	targetUrl, err := url.Parse(opts.Url)
	if err != nil {
		return nil, err
	}

	if _, err := net.ResolveIPAddr("ip", targetUrl.Hostname()); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(string(opts.Method), targetUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "maddsua/pulse")

	if opts.Headers != nil {
		for key, val := range opts.Headers {

			if strings.ToLower(key) == "host" {
				slog.Info("Overriding request host header",
					slog.String("for", label),
					slog.String("to", val))
				req.Host = val
			}

			req.Header.Set(key, val)
		}
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	if opts.Proxy != "" {

		if len(proxies) == 0 {
			return nil, errors.New("no proxies defined in the config")
		}

		proxyCfg, has := proxies[opts.Proxy]
		if !has || proxyCfg == nil {
			return nil, errors.New("proxy tag not found")
		}

		proxyUrl, err := url.Parse(proxyCfg.Url)
		if err != nil {
			return nil, fmt.Errorf("proxy url invalid: %s", err.Error())
		}

		dialer, err := NewSocksProxyDialer(proxyUrl.Host, proxyUrl.User)
		if err != nil {
			return nil, fmt.Errorf("failed to create proxy dialer: %s", err.Error())
		}

		transport.DialContext = dialer.DialContext
	}

	return &httpProbeTask{
		nextRun:  time.Now().Add(time.Second * time.Duration(opts.Interval)),
		timeout:  time.Second * time.Duration(opts.Timeout),
		interval: time.Second * time.Duration(opts.Interval),
		req:      req,
		label:    label,
		client:   &http.Client{Transport: transport},
	}, nil
}

type httpProbeTask struct {
	nextRun  time.Time
	locked   bool
	label    string
	timeout  time.Duration
	interval time.Duration
	req      *http.Request
	client   *http.Client
}

func (this *httpProbeTask) Label() string {
	return this.label
}

func (this *httpProbeTask) Interval() time.Duration {
	return this.interval
}

func (this *httpProbeTask) Ready() bool {
	return !this.locked && time.Now().After(this.nextRun)
}

func (this *httpProbeTask) Do(ctx context.Context, storageDriver storage.Storage) error {

	this.locked = true

	defer func() {
		this.nextRun = time.Now().Add(this.interval)
		this.locked = false
	}()

	started := time.Now()

	ctx, cancel := context.WithTimeout(ctx, this.timeout)
	defer cancel()

	resp, err := this.client.Do(this.req.Clone(ctx))
	if err != nil {

		elapsed := time.Since(started)

		slog.Debug("upd http request failed:",
			slog.String("err", err.Error()),
			slog.Duration("after", elapsed))

		return this.dispatchEntry(storageDriver, storage.PulseEntry{
			Label:      this.label,
			Time:       started,
			Status:     storage.ServiceStatusDown,
			Elapsed:    elapsed,
			HttpStatus: null.IntFrom(this.connErrCode(err)),
			LatencyMs:  -1,
		})
	}

	defer resp.Body.Close()

	if !this.isOkStatus(resp.StatusCode) {
		return this.dispatchEntry(storageDriver, storage.PulseEntry{
			Label:      this.label,
			Time:       started,
			Status:     storage.ServiceStatusDown,
			HttpStatus: null.IntFrom(int64(resp.StatusCode)),
			Elapsed:    time.Since(started),
			LatencyMs:  -1,
		})
	}

	return this.dispatchEntry(storageDriver, storage.PulseEntry{
		Label:      this.label,
		Time:       started,
		Status:     storage.ServiceStatusUp,
		HttpStatus: null.IntFrom(int64(resp.StatusCode)),
		Elapsed:    time.Since(started),
		LatencyMs:  int(time.Since(started).Milliseconds()),
	})
}

func (this *httpProbeTask) dispatchEntry(storageDriver storage.Storage, entry storage.PulseEntry) error {

	slog.Debug("upd http "+this.label,
		slog.String("status", entry.Status.String()),
		slog.Int("http_status", int(entry.HttpStatus.Int64)),
		slog.Duration("elapsed", entry.Elapsed))

	return storageDriver.PushUptime(entry)
}

func (this *httpProbeTask) isOkStatus(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusBadRequest
}

func (this *httpProbeTask) connErrCode(err error) int64 {

	//	This is only needed to indicate a server error status,
	//	which is a higher value than any of the actual valid http statues.
	//	The number itself is taken from websocket close codes (1012/Service Restart)

	switch {
	case strings.HasPrefix(err.Error(), "socks connect"):
		return 1014
	default:
		return 1012
	}
}
