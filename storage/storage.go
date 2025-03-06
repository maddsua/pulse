package storage

import (
	"time"

	"github.com/guregu/null"
)

type Storage interface {
	PushUptime(entry UptimeEntry) error
	QueryUptimeRange(from time.Time, to time.Time) ([]UptimeEntry, error)
	Close() error
}

type ServiceStatus int

const (
	ServiceStatusUp         = 1
	ServiceStatusDown       = 2
	ServiceStatusUpString   = "up"
	ServiceStatusDownString = "down"
)

func (this ServiceStatus) String() string {
	switch this {
	case ServiceStatusUp:
		return ServiceStatusUpString
	case ServiceStatusDown:
		return ServiceStatusDownString
	default:
		return ""
	}
}

func ParseServiceStatus(token string) ServiceStatus {
	switch token {
	case ServiceStatusUpString:
		return ServiceStatusUp
	default:
		return ServiceStatusDown
	}
}

type UptimeEntry struct {
	ID         null.Int
	Time       time.Time
	Label      string
	Status     ServiceStatus
	HttpStatus null.Int
	Elapsed    time.Duration
	LatencyMs  int
}
