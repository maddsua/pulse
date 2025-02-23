package main

import (
	"database/sql"
	"time"
)

type Storage interface {
	Push(entry PulseEntry) error
	QueryRange(from time.Time, to time.Time) ([]PulseEntry, error)
	Close() error
}

type ServiceStatus int

const (
	ServiceStatusUp   = 1
	ServiceStatusDown = 0
)

func (this ServiceStatus) String() string {
	switch this {
	case ServiceStatusUp:
		return "up"
	case ServiceStatusDown:
		return "down"
	default:
		return ""
	}
}

type PulseEntry struct {
	ID      sql.NullInt64
	Time    time.Time
	Label   string
	Status  ServiceStatus
	Elapsed time.Duration
}
