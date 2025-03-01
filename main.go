package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/maddsua/pulse/storage"
	sqlite_storage "github.com/maddsua/pulse/storage/sqlite"
	timescale_storage "github.com/maddsua/pulse/storage/timescale"
	"gopkg.in/yaml.v3"
)

func main() {

	godotenv.Load()

	flagDebug := flag.Bool("debug", false, "Show debug logging")
	flagConfigFile := flag.String("config", "./pulse.config.yml", "Set config value path")
	flagDataDir := flag.String("data", "./data", "Data directory location")
	flagLogFmt := flag.String("logfmt", "", "Log format: json|null")
	flag.Parse()

	if strings.ToLower(os.Getenv("LOG_FMT")) == "json" || strings.ToLower(*flagLogFmt) == "json" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	}

	slog.Info("Starting pulse service")

	if *flagDebug || strings.ToLower(os.Getenv("LOG_LEVEL")) == "debug" {
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

	if err := cfg.Validate(); err != nil {
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

	var storage storage.Storage

	if val := os.Getenv("DATABASE_URL"); val != "" {

		slog.Info("$DATABASE_URL is provided, overriding the default storage driver")

		driver, err := timescale_storage.NewTimescaleStorage(val)
		if err != nil {
			slog.Error("Failed to initialize timescale storage",
				slog.String("err", err.Error()))
			os.Exit(1)
		}
		storage = driver

	} else {

		driver, err := sqlite_storage.NewSqliteStorage(*flagDataDir)
		if err != nil {
			slog.Error("Failed to initialize sqlite3 storage",
				slog.String("err", err.Error()))
			os.Exit(1)
		}
		storage = driver
	}

	defer storage.Close()

	var serveMux *http.ServeMux

	if cfg.Exporters.Series {

		const handlerPath = "/exporters/series"

		slog.Info("Series exporter enabled",
			slog.String("path", handlerPath))

		exporter := &SeriesExporter{Storage: storage}

		if serveMux == nil {
			serveMux = &http.ServeMux{}
		}

		serveMux.Handle(handlerPath, exporter)
	}

	go waitForExitSignal(cancel)

	if serveMux != nil {
		go startApiServer(ctx, serveMux)
	}

	taskhost := TaskHost{
		Storage: storage,
		Tasks:   tasks,
	}

	slog.Info("Starting tasks now")

	taskhost.Run(ctx)
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

			task, err := NewHttpTask(label, *item.Http, cfg.Proxies)
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

func startApiServer(ctx context.Context, mux *http.ServeMux) {

	port := os.Getenv("PORT")
	if _, err := strconv.Atoi(port); err != nil || port == "" {
		port = "7200"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	slog.Info("Starting API server now",
		slog.String("addr", srv.Addr))

	go func() {
		if err := srv.ListenAndServe(); err != nil && ctx.Err() == nil {
			slog.Error("api server error",
				slog.String("err", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancelShutdownCtx := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancelShutdownCtx()

	_ = srv.Shutdown(shutdownCtx)
}

func waitForExitSignal(cancel context.CancelFunc) {

	exit := make(chan os.Signal, 2)
	signal.Notify(exit, syscall.SIGINT, syscall.SIGTERM)

	<-exit

	slog.Info("Shutting down...")

	cancel()
}
