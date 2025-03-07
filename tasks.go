package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/maddsua/pulse/config"
	"github.com/maddsua/pulse/probes"
	"github.com/maddsua/pulse/storage"
)

type ProbeTask interface {
	Ready() bool
	Type() string
	Label() string
	Interval() time.Duration
	Do(ctx context.Context, storage storage.Storage) error
}

type TaskHost struct {
	Tasks   []ProbeTask
	Storage storage.Storage
	Autorun bool
	ticker  *time.Ticker
}

func (this *TaskHost) Run(ctx context.Context) {

	if this.ticker != nil {
		panic("TaskHost.Do() called more than once")
	}

	this.ticker = time.NewTicker(time.Second)

	var execTask = func(task ProbeTask) {

		slog.Debug("exec "+task.Label(),
			slog.Time("next_run", time.Now().Add(task.Interval())))

		if err := task.Do(ctx, this.Storage); err != nil {
			slog.Error("Probe returned error",
				slog.String("label", task.Label()),
				slog.String("type", task.Type()),
				slog.String("err", err.Error()))
		}
	}

	var invokeTasks = func() {
		for _, task := range this.Tasks {
			if task.Ready() {
				go execTask(task)
			}
		}
	}

	if this.Autorun {
		for _, task := range this.Tasks {
			go execTask(task)
		}
	}

	for {
		select {
		case <-this.ticker.C:
			invokeTasks()
		case <-ctx.Done():
			this.ticker.Stop()
			return
		}
	}
}

func CreateProbeTasks(cfg config.RootConfig) ([]ProbeTask, error) {

	var tasks []ProbeTask

	for key, item := range cfg.Probes {

		uptimeChecks := item.UptimeChecks()

		if item.Http != nil {

			label := key
			if uptimeChecks > 1 {
				label += "-http"
			}

			task, err := probes.NewHttpProbe(label, *item.Http, cfg.Proxies)
			if err != nil {
				return nil, fmt.Errorf("task '%s': %s", label, err.Error())
			}

			slog.Info("Added http probe task",
				slog.String("label", label),
				slog.String("method", string(item.Http.Method)),
				slog.String("url", item.Http.Url),
				slog.Duration("interval", task.Interval()),
				slog.Time("next_run", time.Now().Add(task.Interval())))

			tasks = append(tasks, task)
		}

	}

	return tasks, nil
}
