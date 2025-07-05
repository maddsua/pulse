package main

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/maddsua/pulse"
)

type StdoutWriter struct {
}

func (this *StdoutWriter) Type() string {
	return "stdout"
}

func (this *StdoutWriter) Version() string {
	return "x"
}

func (this *StdoutWriter) WriteUptime(ctx context.Context, entry pulse.UptimeEntry) error {

	status := "<nil>"
	if entry.HttpStatus != nil {
		status = strconv.Itoa(*entry.HttpStatus)
	}

	host := "<nil>"
	if entry.Host != nil {
		host = *entry.Host
	}

	tlsVersion := "<nil>"
	if entry.TlsVersion != nil {
		tlsVersion = strconv.Itoa(*entry.TlsVersion)
	}

	slog.Info("STDOUT Uptime",
		slog.String("label", entry.Label),
		slog.Bool("ok", entry.Up),
		slog.Duration("elapsed", entry.ProbeElapsed),
		slog.String("host", host),
		slog.Any("latency", entry.Latency),
		slog.String("http_status", status),
		slog.String("tls_version", tlsVersion))
	return nil
}
