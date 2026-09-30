// Package httpapi implements the incoming HTTP/JSON adapter of the API Service.
package httpapi

import (
	"encoding/json"
	"net/http"

	httptransport "github.com/F1ameX/RepoPulse/project/backend/internal/pkg/adapter/in/http"
)

type errorResponse struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func NewHandler() http.Handler {
	mux := http.NewServeMux()
	route := func(path, method string, handler http.HandlerFunc) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != method && !(method == http.MethodGet && r.Method == http.MethodHead) {
				allow := method
				if method == http.MethodGet {
					allow += ", HEAD"
				}
				w.Header().Set("Allow", allow)
				writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
				return
			}
			handler(w, r)
		})
	}

	route("/health", http.MethodGet, httptransport.Health)
	route("/api/v1/analyses", http.MethodPost, notImplemented)
	route("/api/v1/analyses/{analysis_id}", http.MethodGet, notImplemented)
	route("/api/v1/reports/{report_id}", http.MethodGet, notImplemented)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Route not found")
	})

	return httptransport.WithRequestLogging(mux)
}

func notImplemented(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Analysis and report services are not implemented yet")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: apiError{
		Code: code, Message: message, RequestID: w.Header().Get("X-Request-ID"),
	}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// All response values are JSON-safe structs. A write failure means the client
	// disconnected; the response headers have already been sent.
	_ = json.NewEncoder(w).Encode(value)
}
