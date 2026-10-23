package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
)

func TestHTTPServerDoesNotBypassMissingAuthIntegration(t *testing.T) {
	server := newHTTPServer(context.Background(), Config{})
	for _, tc := range []struct {
		cookie, code string
		status       int
	}{
		{"", "SESSION_REQUIRED", http.StatusUnauthorized},
		{"repopulse_guest=opaque", "NOT_IMPLEMENTED", http.StatusNotImplemented},
		{"repopulse_session=opaque", "NOT_IMPLEMENTED", http.StatusNotImplemented},
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/repositories/validate", strings.NewReader(`{"repository_url":"https://github.com/owner/repo"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Cookie", tc.cookie)
		recorder := httptest.NewRecorder()
		server.Handler.ServeHTTP(recorder, request)
		if recorder.Code != tc.status || !strings.Contains(recorder.Body.String(), `"code":"`+tc.code+`"`) {
			t.Fatalf("unexpected response without Auth integration: %d %s", recorder.Code, recorder.Body.String())
		}
	}
}

func TestRequestContextKeepsLoggerWithoutProcessCancellation(t *testing.T) {
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(logger.With(context.Background(), log))
	defer cancel()

	type observation struct {
		log *slog.Logger
		err error
	}
	observed := make(chan observation, 1)
	server := httptest.NewUnstartedServer(nil)
	server.Config = newHTTPServer(ctx, Config{})
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed <- observation{logger.From(r.Context()), r.Context().Err()}
		w.WriteHeader(http.StatusNoContent)
	})
	server.Start()
	defer server.Close()

	// The lifecycle code controls Shutdown; process cancellation alone must not
	// cancel request contexts before active handlers have had time to drain.
	cancel()
	response, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	got := <-observed
	if got.log != log || got.err != nil {
		t.Fatalf("request lost its logger or inherited process cancellation: %+v", got)
	}
}
