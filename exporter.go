package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/maddsua/pulse/storage"
)

type SeriesExporter struct {
	Storage storage.Storage
}

func (this *SeriesExporter) ServeHTTP(wrt http.ResponseWriter, req *http.Request) {

	rangeFrom := time.Now().Add(-6 * time.Hour)
	rangeTo := time.Now()

	var handleInvalidInput = func(err error) {
		wrt.WriteHeader(http.StatusBadRequest)
		wrt.Write([]byte("invald query intput: " + err.Error()))
	}

	if val := req.URL.Query().Get("from"); val != "" {
		point, err := time.Parse(time.RFC3339, val)
		if err != nil {
			handleInvalidInput(errors.New("invalid 'from' parameter format: " + err.Error()))
			return
		}
		rangeFrom = point
	}

	if val := req.URL.Query().Get("to"); val != "" {
		point, err := time.Parse(time.RFC3339, val)
		if err != nil {
			handleInvalidInput(errors.New("invalid 'to' parameter format: " + err.Error()))
			return
		}
		rangeTo = point
	}

	entries, err := this.Storage.QueryUptimeRange(rangeFrom, rangeTo)
	if err != nil {
		slog.Error("Failed to query data for series exporter",
			slog.String("err", err.Error()))
		return
	}

	result := make([]map[string]any, len(entries))
	for idx, val := range entries {
		result[idx] = map[string]any{
			"time":        val.Time.Format(time.RFC3339),
			"label":       val.Label,
			"status":      val.Status,
			"http_status": val.HttpStatus.Ptr(),
			"elapsed_ms":  val.Elapsed.Milliseconds(),
			"latency_ms":  val.LatencyMs,
		}
	}

	wrt.Header().Set("content-type", "application/json")

	if err := json.NewEncoder(wrt).Encode(result); err != nil {
		slog.Error("Failed to serialize series exporter data",
			slog.String("err", err.Error()))
		return
	}
}
