// Package httptransport provides shared operational HTTP handlers and logging.
package httptransport

import (
	"encoding/json"
	"net/http"
)

// NewHealthHandler exposes only process liveness, not a service's domain API.
func NewHealthHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", Health)
	return WithRequestLogging(mux)
}

func Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(struct {
		Status string `json:"status"`
	}{Status: "ok"})
}
