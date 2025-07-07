package pulse

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/proxy"
)

type HttpProbe struct {
	HttpProbeOptions

	Label  string
	Writer StorageWriter

	locked      atomic.Bool
	nextExec    time.Time
	proxyDialer proxy.ContextDialer
	client      *http.Client
	req         *http.Request
}

type HttpProbeOptions struct {
	Interval time.Duration     `yaml:"interval" json:"interval"`
	Timeout  time.Duration     `yaml:"timeout" json:"timeout"`
	Url      string            `yaml:"url" json:"url"`
	Method   string            `yaml:"method" json:"method"`
	Headers  map[string]string `yaml:"headers" json:"headers"`
	ProxyUrl string            `yaml:"proxy_url" json:"proxy_url"`
	Retries  int               `yaml:"retries" json:"retries"`
}

func (this *HttpProbe) ID() string {
	return this.Label
}

func (this *HttpProbe) Type() string {
	return "http"
}

func (this *HttpProbe) Version() string {
	return "h3"
}

func (this *HttpProbe) validateConfig() error {

	switch {
	case this.Label == "":
		return errors.New("label is empty")
	case this.Url == "":
		return errors.New("empty url")
	}

	if this.Interval <= time.Second {
		this.Interval = time.Minute
	}

	this.Method = strings.ToUpper(this.Method)

	switch this.Method {
	case http.MethodGet, http.MethodOptions, http.MethodHead, http.MethodPost:
		break

	case http.MethodConnect, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return fmt.Errorf("http method '%s' not allowed", this.Method)

	default:
		this.Method = http.MethodGet
	}

	return nil
}

func (this *HttpProbe) Ready() (bool, error) {

	if err := this.validateConfig(); err != nil {
		return false, err
	}

	if this.Writer == nil {
		return false, errors.New("writer is nil")
	}

	//	initialize proxy state if provided
	if this.HttpProbeOptions.ProxyUrl != "" && this.proxyDialer == nil {

		dialer, err := getProxyUrlDialer(this.HttpProbeOptions.ProxyUrl)
		if err != nil {
			return false, fmt.Errorf("proxy_url: %v", err)
		}

		this.client = &http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext:     dialer.DialContext,
		}}
		this.proxyDialer = dialer
	}

	//	 initialize client
	if this.client == nil {
		this.client = &http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}}
	}

	//	init request
	if this.req == nil {

		method := http.MethodGet
		if this.HttpProbeOptions.Method != "" {
			method = strings.ToUpper(this.HttpProbeOptions.Method)
		}

		reqUrl, err := url.Parse(this.HttpProbeOptions.Url)
		if err != nil {
			return false, fmt.Errorf("url.Parse: %v", err)
		}

		if reqUrl.Scheme == "" {
			reqUrl.Scheme = "http"
		}

		slog.Debug("http probe set remote",
			slog.String("remote", reqUrl.String()),
			slog.String("label", this.Label))

		req, err := http.NewRequest(method, reqUrl.String(), nil)
		if err != nil {
			return false, fmt.Errorf("http.NewRequest: %v", err)
		}

		req.Header.Set("User-Agent", "maddsua/pulse")

		if this.HttpProbeOptions.Headers != nil {
			for key, val := range this.HttpProbeOptions.Headers {
				if strings.ToLower(key) == "host" {
					req.Host = val
				}
				req.Header.Set(key, val)
			}
		}

		this.req = req
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

func (this *HttpProbe) Exec(ctx context.Context) error {

	if _, err := this.Ready(); err != nil {
		return err
	}

	if !this.locked.CompareAndSwap(false, true) {
		return errors.New("task locked")
	}
	defer this.locked.Store(false)

	type responseStatus struct {
		Elapsed    time.Duration
		Status     *int
		TlsVersion *int
		Err        error
	}

	var fetchStatus = func(ctx context.Context) (*responseStatus, error) {

		started := time.Now()

		resp, err := this.client.Do(this.req.Clone(ctx))
		if err != nil {

			if isProxyError(err) {
				return nil, err
			}

			return &responseStatus{
				Elapsed: time.Since(started),
				Err:     err,
			}, nil
		}

		resp.Body.Close()

		if resp.TLS != nil {
			version := extractTlsVersion(resp.TLS.Version)
			return &responseStatus{
				Elapsed:    time.Since(started),
				Status:     &resp.StatusCode,
				TlsVersion: &version,
			}, nil
		}

		return &responseStatus{
			Elapsed: time.Since(started),
			Status:  &resp.StatusCode,
		}, nil
	}

	var isOkStatus = func(val int) bool {
		return val >= http.StatusOK && val <= http.StatusIMUsed
	}

	timeout := 10 * time.Second
	if this.HttpProbeOptions.Timeout > 0 {
		timeout = this.HttpProbeOptions.Timeout
	}

	requestCtx, cancelRequests := context.WithTimeout(ctx, timeout)
	defer cancelRequests()

	started := time.Now()

	status, err := fetchStatus(requestCtx)
	if err != nil {
		return err
	}

	if status.Err != nil && this.HttpProbeOptions.Retries > 0 {
		for n := 0; n < this.HttpProbeOptions.Retries && requestCtx.Err() == nil; n++ {
			if status, err = fetchStatus(requestCtx); err != nil {
				return err
			} else if status.Err == nil {
				break
			}
		}
	}

	entry := UptimeEntry{
		Label:        this.Label,
		Timestamp:    time.Now(),
		ProbeType:    this.Type(),
		ProbeElapsed: time.Since(started),
		TlsVersion:   status.TlsVersion,
		HttpStatus:   status.Status,
	}

	if addr, err := net.ResolveIPAddr("ip", this.req.Host); err == nil {
		host := addr.IP.String()
		entry.Host = &host
	}

	if status.Status != nil && isOkStatus(*status.Status) {
		entry.Up = true
		entry.Latency = &status.Elapsed
	}

	if err := this.Writer.WriteUptime(ctx, entry); err != nil {
		return fmt.Errorf("storage.WriteUptime: %v", err)
	}

	return nil
}

func getProxyUrlDialer(urlString string) (proxy.ContextDialer, error) {

	if strings.HasPrefix(urlString, "$") {
		urlString = os.Getenv(strings.ToUpper(urlString[1:]))
	}

	if urlString == "" {
		return nil, errors.New("empty proxy url")
	}

	proxyUrl, err := url.Parse(urlString)
	if err != nil {
		return nil, fmt.Errorf("url.Parse: %v", err)
	}

	switch strings.ToLower(proxyUrl.Scheme) {

	case "socks", "socks5":

		var proxyAuth *proxy.Auth
		if username := proxyUrl.User.Username(); username != "" {
			pass, _ := proxyUrl.User.Password()
			proxyAuth = &proxy.Auth{User: username, Password: pass}
		}

		dialer, err := proxy.SOCKS5("tcp", proxyUrl.Host, proxyAuth, proxy.Direct)
		if err != nil {
			return nil, err
		}

		return dialer.(proxy.ContextDialer), nil

	default:
		return nil, fmt.Errorf("unsupported proxy protocol: %v", proxyUrl.Scheme)
	}
}

func isProxyError(err error) bool {

	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "socks connect tcp")
}

func extractTlsVersion(version uint16) int {
	switch version {
	case tls.VersionSSL30:
		return 300
	case tls.VersionTLS10:
		return 100
	case tls.VersionTLS11:
		return 110
	case tls.VersionTLS12:
		return 120
	case tls.VersionTLS13:
		return 130
	default:
		return int(version)
	}
}
