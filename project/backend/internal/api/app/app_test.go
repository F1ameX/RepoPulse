package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
)

func TestServerHealthAndShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil)).With("service", "api-service")
	ctx, cancel := context.WithCancel(logger.With(context.Background(), log))
	defer cancel()
	done := make(chan error, 1)
	go func() {
		server := newHTTPServer(ctx, Config{})
		done <- serve(ctx, listener, server, time.Second)
	}()

	client := &http.Client{Timeout: 3 * time.Second}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request, err := http.NewRequest(method, "http://"+listener.Addr().String()+"/health", nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("health: status=%d, read error=%v", response.StatusCode, readErr)
		}
		if method == http.MethodHead && len(body) != 0 {
			t.Fatalf("HEAD returned a body: %s", body)
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
	requestsLogged := 0
	decoder := json.NewDecoder(&logs)
	for {
		var entry struct {
			Service   string `json:"service"`
			Message   string `json:"msg"`
			RequestID string `json:"request_id"`
		}
		if err := decoder.Decode(&entry); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if entry.Service != "api-service" {
			t.Fatalf("context logger was lost: %+v", entry)
		}
		if entry.Message == "HTTP request" {
			requestsLogged++
			if entry.RequestID == "" {
				t.Fatal("HTTP request log has no request ID")
			}
		}
	}
	if requestsLogged != 2 {
		t.Fatalf("got %d request logs, want 2", requestsLogged)
	}
	if response, err := client.Get("http://" + listener.Addr().String() + "/health"); err == nil {
		response.Body.Close()
		t.Fatal("server still accepts requests after shutdown")
	}
}

func TestRunReportsOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx := context.Background()
	server := newHTTPServer(ctx, Config{HTTPAddr: listener.Addr().String()})
	err = runHTTP(ctx, server, time.Second)
	if err == nil || !strings.Contains(err.Error(), "listen") {
		t.Fatalf("expected listen error, got %v", err)
	}
}
