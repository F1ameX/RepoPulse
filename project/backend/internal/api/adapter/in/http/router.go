// Package httpapi implements the incoming HTTP/JSON adapter of the API Service.
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/in/http/dto"
)

type errorResponse = dto.ErrorResponse
type apiError = dto.APIError
type fieldError = dto.FieldError

func NewHandler(validator RepositoryValidator, resolver ContextResolver) http.Handler {
	return NewAnalysisHandler(validator, resolver, nil, "")
}

func NewAnalysisHandler(validator RepositoryValidator, resolver ContextResolver, analyses AnalysisService, allowedOrigin string) http.Handler {
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

	route("/health", http.MethodGet, health)
	if analyses == nil {
		route("/api/v1/analyses", http.MethodPost, notImplemented)
		route("/api/v1/analyses/{analysis_id}", http.MethodGet, notImplemented)
	} else {
		route("/api/v1/analyses", http.MethodPost, createAnalysis(analyses, resolver, allowedOrigin))
		route("/api/v1/analyses/{analysis_id}", http.MethodGet, getAnalysis(analyses, resolver))
	}
	route("/api/v1/reports/{report_id}", http.MethodGet, notImplemented)
	route("/api/v1/reports", http.MethodGet, notImplemented)
	route("/api/v1/repositories/validate", http.MethodPost, validateRepository(validator, resolver))
	route("/api/v1/auth/register", http.MethodPost, notImplemented)
	route("/api/v1/auth/login", http.MethodPost, notImplemented)
	route("/api/v1/auth/guest-session", http.MethodPost, notImplemented)
	route("/api/v1/auth/session", http.MethodGet, notImplemented)
	route("/api/v1/auth/logout", http.MethodPost, notImplemented)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Route not found")
	})

	return withRequestLogging(mux)
}

func notImplemented(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Analysis and report services are not implemented yet")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: apiError{
		Code: code, Message: message, TraceID: w.Header().Get("X-Request-ID"),
		FieldErrors: []fieldError{},
	}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// All response values are JSON-safe structs. A write failure means the client
	// disconnected; the response headers have already been sent.
	_ = json.NewEncoder(w).Encode(value)
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, struct {
		Status string `json:"status"`
	}{Status: "ok"})
}
