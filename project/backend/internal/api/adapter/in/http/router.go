// Package httpapi implements the incoming HTTP/JSON adapter of the API Service.
package httpapi

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/libs/logger"
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

	route("/health", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, struct {
			Status string `json:"status"`
		}{Status: "ok"})
	})
	route("/api/v1/analyses", http.MethodPost, notImplemented)
	route("/api/v1/analyses/{analysis_id}", http.MethodGet, notImplemented)
	route("/api/v1/reports/{report_id}", http.MethodGet, notImplemented)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Route not found")
	})

	return withRequestLogging(mux)
}

func withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := rand.Text()
		log := logger.From(r.Context()).With("request_id", requestID)
		r = r.WithContext(logger.With(r.Context(), log))
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		response := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(response, r)
		logger.From(r.Context()).InfoContext(r.Context(), "HTTP request",
			"method", r.Method,
			"path", r.URL.Path, "status", response.status,
			"duration_ms", time.Since(start).Milliseconds())
	})
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

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *responseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
