package httpapi

import (
	"crypto/rand"
	"net/http"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/libs/logger"
)

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
			"method", r.Method, "path", r.URL.Path, "status", response.status,
			"duration_ms", time.Since(start).Milliseconds())
	})
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
