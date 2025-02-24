package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/guregu/null"
)

func NewHttpTask(label string, opts HttpProbeConfig) (*httpProbeTask, error) {

	url, err := url.Parse(opts.Url)
	if err != nil {
		return nil, err
	}

	if _, err := net.ResolveIPAddr("ip", url.Hostname()); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(string(opts.Method), url.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "maddsua/pulse")

	if opts.Headers != nil {
		for key, val := range opts.Headers {
			req.Header.Set(key, val)
		}
	}

	return &httpProbeTask{
		nextRun:  time.Now().Add(time.Second * time.Duration(opts.Interval)),
		timeout:  time.Second * time.Duration(opts.Timeout),
		interval: time.Second * time.Duration(opts.Interval),
		req:      req,
		label:    label,
	}, nil
}

type httpProbeTask struct {
	nextRun  time.Time
	locked   bool
	label    string
	timeout  time.Duration
	interval time.Duration
	req      *http.Request
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

func (this *httpProbeTask) Do(ctx context.Context, storage Storage) error {

	this.locked = true

	defer func() {
		this.nextRun = time.Now().Add(this.interval)
		this.locked = false
	}()

	started := time.Now()

	ctx, cancel := context.WithTimeout(ctx, this.timeout)
	defer cancel()

	resp, err := http.DefaultClient.Do(this.req.Clone(ctx))
	if err != nil {
		return this.dispatchEntry(storage, PulseEntry{
			Label:     this.label,
			Time:      started,
			Status:    ServiceStatusDown,
			Elapsed:   time.Since(started),
			LatencyMs: -1,
		})
	}

	defer resp.Body.Close()

	if !this.isOkStatus(resp.StatusCode) {
		return this.dispatchEntry(storage, PulseEntry{
			Label:      this.label,
			Time:       started,
			Status:     ServiceStatusDown,
			HttpStatus: null.IntFrom(int64(resp.StatusCode)),
			Elapsed:    time.Since(started),
			LatencyMs:  -1,
		})
	}

	return this.dispatchEntry(storage, PulseEntry{
		Label:      this.label,
		Time:       started,
		Status:     ServiceStatusUp,
		HttpStatus: null.IntFrom(int64(resp.StatusCode)),
		Elapsed:    time.Since(started),
		LatencyMs:  int(time.Since(started).Milliseconds()),
	})
}

func (this *httpProbeTask) dispatchEntry(storage Storage, entry PulseEntry) error {

	slog.Debug("Http probe: Update",
		slog.String("label", this.label),
		slog.String("status", entry.Status.String()),
		slog.Int("http_status", int(entry.HttpStatus.Int64)),
		slog.Duration("elapsed", entry.Elapsed))

	return storage.Push(entry)
}

func (this *httpProbeTask) isOkStatus(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusBadRequest
}
