package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	pulse "github.com/maddsua/pulse"
)

type CliFlags struct {
	Cfg      *string
	Debug    *bool
	JsonLogs *bool
}

func main() {

	godotenv.Load()

	cli := CliFlags{
		Cfg:      flag.String("cfg", "", "config file location"),
		Debug:    flag.Bool("debug", false, "enable debug logging"),
		JsonLogs: flag.Bool("json_logs", false, "log in json format"),
	}
	flag.Parse()

	if os.Getenv("DEBUG") == "true" || *cli.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	if os.Getenv("LOGFMT") == "json" || *cli.JsonLogs {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	}

	if *cli.Cfg == "" {
		if loc, has := FindConfig([]string{
			"./pulse.yml",
			"/etc/mws/pulse/pulse.yml",
		}); has {
			cli.Cfg = &loc
		}
	}

	if *cli.Cfg == "" {
		slog.Error("No config files found")
		os.Exit(1)
	}

	cfg, err := LoadConfigFile(*cli.Cfg)
	if err != nil {
		slog.Error("Failed to load config",
			slog.String("err", err.Error()))
		os.Exit(1)
	}

	var storageDriver pulse.StorageWriter

	if val := os.Getenv("TIMESCALE_URL"); val != "" {
		timescale, err := pulse.NewTimescaleStorage(val)
		if err != nil {
			slog.Error("Failed to set up timescale storage",
				slog.String("err", err.Error()))
			os.Exit(1)
		}
		storageDriver = timescale
		defer timescale.Close()
	} else if val := os.Getenv("PUSHGATEWAY_URL"); val != "" {
		pushgateway, err := pulse.NewPushgatewayStorage(val)
		if err != nil {
			slog.Error("Failed to set up prometheus push gateway storage",
				slog.String("err", err.Error()))
			os.Exit(1)
		}
		storageDriver = pushgateway
	} else if val := os.Getenv("INFLUXDB_URL"); val != "" {
		influx, err := pulse.NewInfluxStorage(val)
		if err != nil {
			slog.Error("Failed to set up influxdb storage",
				slog.String("err", err.Error()))
			os.Exit(1)
		}
		storageDriver = influx
	} else {
		storageDriver = &StdoutWriter{}
	}

	slog.Info("USING STORAGE",
		slog.String("type", storageDriver.Type()),
		slog.String("version", storageDriver.Version()))

	probeLabelMap := map[string]int{}

	var indexLabels = func(labeler Labeler) {
		for _, label := range labeler.Labels() {
			probeLabelMap[label] = probeLabelMap[label] + 1
		}
	}

	var dedupProbeKey = func(label *string, branch string) {

		var isUnique = func(label string) bool {
			return probeLabelMap[label] <= 1
		}

		if isUnique(*label) {
			return
		}

		if newLabel := *label + ":" + branch; isUnique(newLabel) {
			*label = newLabel
			return
		}

		for idx := 1; idx > 0; idx++ {
			newLabel := fmt.Sprintf("%s-%d:%s", *label, idx, branch)
			if isUnique(newLabel) {
				*label = newLabel
				break
			}
		}
	}

	indexLabels(cfg.Probes.Http)
	indexLabels(cfg.Probes.Icmp)

	var probes []Probe

	for key, cfg := range cfg.Probes.Http {

		dedupProbeKey(&key, "http")

		probe := pulse.HttpProbe{
			Label:            key,
			Writer:           storageDriver,
			HttpProbeOptions: cfg,
		}

		if _, err := probe.Ready(); err != nil {
			slog.Error("Failed to load http probe",
				slog.String("key", key),
				slog.String("err", err.Error()))
			os.Exit(1)
		}

		slog.Info("Add http probe",
			slog.String("key", key),
			slog.Duration("interval", probe.HttpProbeOptions.Interval),
			slog.String("url", probe.HttpProbeOptions.Url))

		probes = append(probes, &probe)
	}

	for key, cfg := range cfg.Probes.Icmp {

		dedupProbeKey(&key, "icmp")

		probe := pulse.IcmpProbe{
			Label:            key,
			Writer:           storageDriver,
			IcmpProbeOptions: cfg,
		}

		if _, err := probe.Ready(); err != nil {
			slog.Error("Failed to load icmp probe",
				slog.String("key", key),
				slog.String("err", err.Error()))
			os.Exit(1)
		}

		slog.Info("Add icmp probe",
			slog.String("key", key),
			slog.Duration("interval", probe.IcmpProbeOptions.Interval),
			slog.String("host", probe.IcmpProbeOptions.Host))

		probes = append(probes, &probe)
	}

	ticker := time.NewTicker(time.Second)
	exitCh := make(chan os.Signal, 2)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)

	var invokeProbe = func(probe Probe) {

		started := time.Now()

		if err := probe.Exec(context.Background()); err != nil {
			slog.Error("probe.Exec",
				slog.String("id", probe.ID()),
				slog.String("err", err.Error()))
			return
		}

		slog.Debug("probe.Exec",
			slog.String("id", probe.ID()),
			slog.Duration("t", time.Since(started)))
	}

	if cfg.Autorun {

		slog.Info("Autorun enabled")

		for _, probe := range probes {
			go invokeProbe(probe)
		}
	}

	for {
		select {

		case <-ticker.C:

			for _, probe := range probes {
				if ready, _ := probe.Ready(); ready {
					go invokeProbe(probe)
				}
			}

		case <-exitCh:
			slog.Warn("Shutting down...")
			return
		}
	}
}
