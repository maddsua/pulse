package main

import "context"

type Probe interface {
	ID() string
	Type() string
	Ready() (bool, error)
	Exec(ctx context.Context) error
}
