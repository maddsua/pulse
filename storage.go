package pulse

import (
	"context"
	"time"
)

type StorageWriter interface {
	Type() string
	Version() string
	WriteUptime(ctx context.Context, entry UptimeEntry) error
}

type UptimeEntry struct {
	Label        string
	Timestamp    time.Time
	ProbeType    string
	ProbeElapsed time.Duration
	Up           bool
	Latency      *time.Duration
	HttpStatus   *int
	TlsVersion   *int
	Host         *string
}
