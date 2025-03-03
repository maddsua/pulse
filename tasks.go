package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/maddsua/pulse/storage"
)

type ProbeTask interface {
	Ready() bool
	Label() string
	Interval() time.Duration
	Do(ctx context.Context, storage storage.Storage) error
}

type TaskHost struct {
	Tasks   []ProbeTask
	Storage storage.Storage
	ticker  *time.Ticker
}

func (this *TaskHost) Run(ctx context.Context) {

	if this.ticker != nil {
		panic("TaskHost.Do() called more than once")
	}

	this.ticker = time.NewTicker(time.Second)

	var invokeTask = func(task ProbeTask) {

		slog.Debug("exec "+task.Label(),
			slog.Time("next_run", time.Now().Add(task.Interval())))

		if err := task.Do(ctx, this.Storage); err != nil {
			slog.Error("Probe returned error",
				slog.String("label", task.Label()),
				slog.String("err", err.Error()))
		}
	}

	var updateTasks = func() {
		for _, task := range this.Tasks {
			if task.Ready() {
				go invokeTask(task)
			}
		}
	}

	for {
		select {
		case <-this.ticker.C:
			updateTasks()
		case <-ctx.Done():
			this.ticker.Stop()
			return
		}
	}
}
