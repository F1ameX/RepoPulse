package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httptransport "github.com/F1ameX/RepoPulse/project/backend/internal/pkg/adapter/in/http"
	"github.com/F1ameX/RepoPulse/project/backend/libs/logger"
)

func TestHTTPContract(t *testing.T) {
	handler := NewHandler()
	ctx := logger.With(context.Background(), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	for _, tc := range []struct {
		method, path string
		status       int
		code, allow  string
	}{
		{"GET", "/health", 200, "", ""},
		{"POST", "/api/v1/analyses", 501, "NOT_IMPLEMENTED", ""},
		{"GET", "/api/v1/analyses/A123", 501, "NOT_IMPLEMENTED", ""},
		{"GET", "/api/v1/reports/R123", 501, "NOT_IMPLEMENTED", ""},
		{"GET", "/", 404, "NOT_FOUND", ""},
		{"GET", "/health/extra", 404, "NOT_FOUND", ""},
		{"GET", "/api/v1/analyses/A123/extra", 404, "NOT_FOUND", ""},
		{"GET", "/api/v1/reports", 404, "NOT_FOUND", ""},
		{"GET", "/api/v1/analyses", 405, "METHOD_NOT_ALLOWED", "POST"},
		{"DELETE", "/api/v1/reports/R123", 405, "METHOD_NOT_ALLOWED", "GET, HEAD"},
		{"POST", "/health", 405, "METHOD_NOT_ALLOWED", "GET, HEAD"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequestWithContext(ctx, tc.method, tc.path, nil))
			if recorder.Code != tc.status {
				t.Fatalf("got status %d, want %d: %s", recorder.Code, tc.status, recorder.Body.String())
			}
			if ct := recorder.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
				t.Fatalf("unexpected Content-Type: %q", ct)
			}
			if allow := recorder.Header().Get("Allow"); allow != tc.allow {
				t.Fatalf("Allow = %q, want %q", allow, tc.allow)
			}
			requestID := recorder.Header().Get("X-Request-ID")
			if requestID == "" {
				t.Fatal("missing X-Request-ID")
			}
			if tc.code == "" {
				if strings.TrimSpace(recorder.Body.String()) != `{"status":"ok"}` {
					t.Fatalf("unexpected health response: %s", recorder.Body.String())
				}
				return
			}
			var response errorResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Error.Code != tc.code || response.Error.RequestID != requestID || response.Error.Message == "" {
				t.Fatalf("unexpected error response: %+v", response)
			}
		})
	}
}

func TestRequestIDsMatchLogsAndAreGeneratedPerRequest(t *testing.T) {
	var logs bytes.Buffer
	handler := NewHandler()
	ctx := logger.With(context.Background(), slog.New(slog.NewJSONHandler(&logs, nil)))
	previousID := ""
	for range 2 {
		logs.Reset()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/health?secret=do-not-log", nil)
		request.Header.Set("X-Request-ID", "untrusted-client-id")
		handler.ServeHTTP(recorder, request)
		id := recorder.Header().Get("X-Request-ID")
		if id == "" || id == previousID || id == "untrusted-client-id" {
			t.Fatalf("expected fresh request ID, got %q", id)
		}
		previousID = id
		var entry struct {
			RequestID string `json:"request_id"`
			Status    int    `json:"status"`
			Path      string `json:"path"`
		}
		if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
		if entry.RequestID != id || entry.Status != 200 || entry.Path != "/health" || strings.Contains(logs.String(), "secret") {
			t.Fatalf("unexpected log: %s", logs.String())
		}
	}
}

func TestRequestLoggerIsAvailableToHandlersAndDoesNotLeak(t *testing.T) {
	var logs bytes.Buffer
	baseLog := slog.New(slog.NewJSONHandler(&logs, nil)).With("service", "api-service")
	ctx := logger.With(context.Background(), baseLog)
	handler := httptransport.WithRequestLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.From(r.Context()).InfoContext(r.Context(), "handler log")
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil))
	logger.From(ctx).InfoContext(ctx, "outside request")

	decoder := json.NewDecoder(&logs)
	for _, message := range []string{"handler log", "HTTP request", "outside request"} {
		var entry struct {
			Message   string `json:"msg"`
			Service   string `json:"service"`
			RequestID string `json:"request_id"`
		}
		if err := decoder.Decode(&entry); err != nil {
			t.Fatal(err)
		}
		wantID := recorder.Header().Get("X-Request-ID")
		if message == "outside request" {
			wantID = ""
		}
		if entry.Message != message || entry.Service != "api-service" || entry.RequestID != wantID {
			t.Fatalf("unexpected contextual log: %+v", entry)
		}
	}
}
