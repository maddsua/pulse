package exporters

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func respondInvalidInput(wrt http.ResponseWriter, err error) {
	wrt.WriteHeader(http.StatusBadRequest)
	wrt.Write([]byte("invald query intput: " + err.Error()))
}

func respondData(wrt http.ResponseWriter, data any) {

	wrt.Header().Set("content-type", "application/json")

	jsonEnc := json.NewEncoder(wrt)
	jsonEnc.SetIndent("", "  ")

	if err := jsonEnc.Encode(data); err != nil {
		slog.Error("Failed to serialize exporter data",
			slog.String("err", err.Error()))
		return
	}
}
