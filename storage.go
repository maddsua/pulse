package main

import (
	"time"

	"github.com/guregu/null"
)

type Storage interface {
	Push(entry PulseEntry) error
	QueryRange(from time.Time, to time.Time) ([]PulseEntry, error)
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

type PulseEntry struct {
	ID         null.Int
	Time       time.Time
	Label      string
	Status     ServiceStatus
	HttpStatus null.Int
	Elapsed    time.Duration
	Latency    null.Int
}
