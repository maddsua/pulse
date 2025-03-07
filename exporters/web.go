package exporters

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/maddsua/pulse/storage"
)

type WebExporter struct {
	Storage storage.Storage
	mux     *http.ServeMux
}

func (this *WebExporter) ServeHTTP(wrt http.ResponseWriter, req *http.Request) {

	if this.mux == nil {
		this.mux = http.NewServeMux()
		this.mux.Handle("GET /uptime", http.HandlerFunc(this.handleUptime))
	}

	this.mux.ServeHTTP(wrt, req)
}

func (this *WebExporter) handleUptime(wrt http.ResponseWriter, req *http.Request) {

	rangeFrom := time.Now().Add(-6 * time.Hour)
	rangeTo := time.Now()
	var rangeInterval time.Duration

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

	if val := req.URL.Query().Get("interval"); val != "" {
		interval, err := time.ParseDuration(val)
		if err != nil {
			handleInvalidInput(errors.New("invalid 'interval' parameter format: " + err.Error()))
			return
		}
		rangeInterval = interval
	}

	entries, err := this.Storage.QueryUptimeRange(rangeFrom, rangeTo)
	if err != nil {
		slog.Error("Failed to query data for series exporter",
			slog.String("err", err.Error()))
		return
	}

	if rangeInterval > 0 {
		entries = aggregateUptimeEntries(entries, rangeInterval)
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

	jsonEnc := json.NewEncoder(wrt)
	jsonEnc.SetIndent("", "  ")

	if err := jsonEnc.Encode(result); err != nil {
		slog.Error("Failed to serialize series exporter data",
			slog.String("err", err.Error()))
		return
	}
}
