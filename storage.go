package pulse

import (
	"context"
	"time"
)

type StorageWriter interface {
	//	Returns driver TypeID (usually a database name, like "postgres")
	Type() string
	//	Returns client version (could indicate API version that's being used, or DB migration version)
	Version() string
	//	Write a signel uptime metric
	WriteUptime(ctx context.Context, entry UptimeEntry) error
}

type UptimeEntry struct {
	//	Unique metric label
	Label string
	//	Measurement timestamp
	Timestamp time.Time
	//	Probe type ID (http|icmp|etc)
	ProbeType string
	//	Total time that took the probe to get the measurement
	ProbeElapsed time.Duration
	//	Whether the checked service is up
	Up bool
	//	Service latency (only if is up, otherwise value is nil)
	Latency *time.Duration
	//	Returned http status code (only for http)
	HttpStatus *int
	//	Used TLS version (only for hott)
	TlsVersion *int
	//	Resolved host address
	Host *string
}
