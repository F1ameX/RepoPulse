package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/F1ameX/RepoPulse/project/backend/libs/logger"
)

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
