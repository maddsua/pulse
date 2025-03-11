package probes

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/guregu/null"
	"github.com/maddsua/pulse/config"
	"github.com/maddsua/pulse/storage"
)

func NewHttpProbe(label string, opts config.HttpProbeConfig, proxies config.ProxyConfigMap) (*httpProbe, error) {

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

		proxy, err := loadProxy(opts.Proxy, proxies)
		if err != nil {
			return nil, err
		}

		transport.DialContext = proxy.DialContext
	}

	return &httpProbe{
		probeTask: probeTask{
			BaseProbeConfig: opts.BaseProbeConfig,
			label:           label,
		},
		req:    req,
		client: &http.Client{Transport: transport},
	}, nil
}

type httpProbe struct {
	probeTask
	req    *http.Request
	client *http.Client
}

func (this *httpProbe) Type() string {
	return "http"
}

func (this *httpProbe) Label() string {
	return this.label
}

func (this *httpProbe) Do(ctx context.Context, storageDriver storage.Storage) error {

	if err := this.probeTask.Lock(); err != nil {
		return err
	}

	defer this.probeTask.Unlock()

	started := time.Now()

	reqCtx, cancelReq := context.WithTimeout(ctx, this.BaseProbeConfig.Timeout())
	defer cancelReq()

	resp, err := this.client.Do(this.req.Clone(reqCtx))
	if err != nil {

		elapsed := time.Since(started)

		slog.Debug("upd http request failed:",
			slog.String("err", err.Error()),
			slog.Duration("after", elapsed))

		return this.dispatchEntry(storageDriver, storage.UptimeEntry{
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
		return this.dispatchEntry(storageDriver, storage.UptimeEntry{
			Label:      this.label,
			Time:       started,
			Status:     storage.ServiceStatusDown,
			HttpStatus: null.IntFrom(int64(resp.StatusCode)),
			Elapsed:    time.Since(started),
			LatencyMs:  -1,
		})
	}

	return this.dispatchEntry(storageDriver, storage.UptimeEntry{
		Label:      this.label,
		Time:       started,
		Status:     storage.ServiceStatusUp,
		HttpStatus: null.IntFrom(int64(resp.StatusCode)),
		Elapsed:    time.Since(started),
		LatencyMs:  int(time.Since(started).Milliseconds()),
	})
}

func (this *httpProbe) dispatchEntry(storageDriver storage.Storage, entry storage.UptimeEntry) error {

	slog.Debug("upd http "+this.label,
		slog.String("status", entry.Status.String()),
		slog.Int("http_status", int(entry.HttpStatus.Int64)),
		slog.Duration("elapsed", entry.Elapsed))

	return storageDriver.PushUptime(entry)
}

func (this *httpProbe) isOkStatus(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusBadRequest
}

func (this *httpProbe) connErrCode(err error) int64 {

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
