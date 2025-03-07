package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/maddsua/pulse/config"
	"github.com/maddsua/pulse/exporters"
	"github.com/maddsua/pulse/storage"
	sqlite_storage "github.com/maddsua/pulse/storage/sqlite"
	timescale_storage "github.com/maddsua/pulse/storage/timescale"
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

	cfg, err := config.LoadConfigFile(*flagConfigFile)
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

	tasks, err := CreateProbeTasks(*cfg)
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

	serveMux := &http.ServeMux{}

	if cfg.Exporters.Web.Enabled {

		handlerPath := PrefixPath("/exporters/web")

		slog.Info("Web exporter enabled",
			slog.String("path", handlerPath.String()))

		serveMux.Handle(handlerPath.Path(), http.StripPrefix(handlerPath.String(), &exporters.WebExporter{
			Storage: storage,
		}))
	}

	go waitForExitSignal(cancel)

	if cfg.Exporters.HasHandlers() {
		go startApiServer(ctx, serveMux)
	}

	taskhost := TaskHost{
		Storage: storage,
		Tasks:   tasks,
		Autorun: cfg.Taskhost.Autorun,
	}

	slog.Info("Starting tasks now")

	taskhost.Run(ctx)
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

type PrefixPath string

func (this PrefixPath) Path() string {
	return string(this) + "/"
}

func (this PrefixPath) String() string {
	return string(this)
}
