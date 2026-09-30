package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/config"
)

func TestServerHealthAndShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, listener, config.Config{ShutdownTimeout: time.Second},
			slog.New(slog.NewJSONHandler(io.Discard, nil)))
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
	err = Run(context.Background(), config.Config{HTTPAddr: listener.Addr().String()}, slog.Default())
	if err == nil || !strings.Contains(err.Error(), "listen") {
		t.Fatalf("expected listen error, got %v", err)
	}
}
