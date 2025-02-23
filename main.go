package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

func main() {

	flagDebug := flag.Bool("debug", false, "Show debug logging")
	flagConfigFile := flag.String("config", "./pulse.config.yml", "Set config value path")
	flagDataDir := flag.String("data", "./data", "Data directory location")
	flag.Parse()

	if *flagDebug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
		slog.Debug("Enabled")
	}

	slog.Info("Config file located",
		slog.String("at", *flagConfigFile))

	cfg, err := loadConfigFile(*flagConfigFile)
	if err != nil {
		slog.Error("Failed to load config file",
			slog.String("err", err.Error()))
		os.Exit(1)
	}

	if err := cfg.Valid(); err != nil {
		slog.Error("Failed to validate config file",
			slog.String("err", err.Error()))
		os.Exit(1)
	}

	tasks, err := createProbeTasks(*cfg)
	if err != nil {
		slog.Error("Failed to initialize tasks",
			slog.String("err", err.Error()))
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	storage, err := NewSqliteStorage(*flagDataDir)
	if err != nil {
		slog.Error("Failed to initialize sqlite3 storage",
			slog.String("err", err.Error()))
		os.Exit(1)
	}

	defer storage.Close()

	go func() {

		exit := make(chan os.Signal, 2)
		signal.Notify(exit, syscall.SIGINT, syscall.SIGTERM)

		<-exit
		cancel()
		slog.Info("Shutting down...")
	}()

	taskhost := TaskHost{
		Context: ctx,
		Storage: storage,
		Tasks:   tasks,
	}

	taskhost.Run()
}

func loadConfigFile(path string) (*RootConfig, error) {

	file, err := os.OpenFile(path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %s", err.Error())
	}

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get config file info: %s", err.Error())
	}

	if !info.Mode().IsRegular() {
		return nil, errors.New("failed to read config file: config file must be a regular file")
	}

	var cfg RootConfig

	if strings.HasSuffix(path, ".yml") {
		if err := yaml.NewDecoder(file).Decode(&cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config file: %s", err.Error())
		}
	} else if strings.HasSuffix(path, ".json") {
		if err := json.NewDecoder(file).Decode(&cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config file: %s", err.Error())
		}
	} else {
		return nil, errors.New("unsupported config file format")
	}

	return &cfg, nil
}

func createProbeTasks(cfg RootConfig) ([]ProbeTask, error) {

	var tasks []ProbeTask

	for key, item := range cfg.Probes {

		stacks := item.Stacks()

		if item.Http != nil {

			label := key
			if stacks > 1 {
				label += "-http"
			}

			task, err := NewHttpTask(label, *item.Http)
			if err != nil {
				return nil, fmt.Errorf("task '%s': %s", label, err.Error())
			}

			slog.Info("Added http probe task",
				slog.String("label", label),
				slog.String("url", item.Http.Url),
				slog.Duration("interval", task.Interval()),
				slog.Time("next_run", time.Now().Add(task.Interval())))

			tasks = append(tasks, task)
		}

	}

	return tasks, nil
}
