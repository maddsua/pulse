package main

import (
	"context"
	"log/slog"
	"time"
)

type ProbeTask interface {
	Ready() bool
	Label() string
	Interval() time.Duration
	Do(ctx context.Context, storage Storage) error
}

type TaskHost struct {
	Context context.Context
	Tasks   []ProbeTask
	Storage Storage
	ticker  *time.Ticker
}

func (this *TaskHost) Run() {

	if this.ticker != nil {
		panic("TaskHost.Do() called more than once")
	}

	this.ticker = time.NewTicker(time.Second)

	var invokeTask = func(task ProbeTask) {

		slog.Debug("Invoking probe task",
			slog.String("label", task.Label()),
			slog.Time("next_run", time.Now().Add(task.Interval())))

		if err := task.Do(this.Context, this.Storage); err != nil {
			slog.Error("Proble task error",
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
		case <-this.Context.Done():
			this.ticker.Stop()
			return
		}
	}
}
