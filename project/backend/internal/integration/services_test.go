package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	analysisapp "github.com/F1ameX/RepoPulse/project/backend/internal/analysis/app"
	apiapp "github.com/F1ameX/RepoPulse/project/backend/internal/api/app"
	authapp "github.com/F1ameX/RepoPulse/project/backend/internal/auth/app"
	githubapp "github.com/F1ameX/RepoPulse/project/backend/internal/github/app"
	orchestratorapp "github.com/F1ameX/RepoPulse/project/backend/internal/orchestrator/app"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/config"
	recommendationapp "github.com/F1ameX/RepoPulse/project/backend/internal/recommendation/app"
	reportapp "github.com/F1ameX/RepoPulse/project/backend/internal/report/app"
	sandboxapp "github.com/F1ameX/RepoPulse/project/backend/internal/sandbox/app"
	scoringapp "github.com/F1ameX/RepoPulse/project/backend/internal/scoring/app"
	"github.com/F1ameX/RepoPulse/project/backend/libs/logger"
)

type startupEvent struct {
	Message string `json:"msg"`
	Service string `json:"service"`
	Address string `json:"address"`
}

type startupWriter struct {
	ready chan startupEvent
}

func (w startupWriter) Write(data []byte) (int, error) {
	var event startupEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return 0, err
	}
	if event.Message == "HTTP server listening" {
		select {
		case w.ready <- event:
		default:
		}
	}
	return len(data), nil
}

func TestAllServiceApplications(t *testing.T) {
	// Environment belongs to the test process. Restore every original value at
	// cleanup so a developer's overrides do not hide broken service defaults.
	for _, key := range []string{
		"HTTP_ADDR", "LOG_LEVEL", "HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT",
		"HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT", "SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	services := []struct {
		name        string
		defaultAddr string
		load        func() (config.Config, error)
		run         func(context.Context, config.Config) error
	}{
		{apiapp.Name, "127.0.0.1:8080", apiapp.LoadConfig, apiapp.Run},
		{authapp.Name, "127.0.0.1:8081", authapp.LoadConfig, authapp.Run},
		{orchestratorapp.Name, "127.0.0.1:8082", orchestratorapp.LoadConfig, orchestratorapp.Run},
		{githubapp.Name, "127.0.0.1:8083", githubapp.LoadConfig, githubapp.Run},
		{sandboxapp.Name, "127.0.0.1:8084", sandboxapp.LoadConfig, sandboxapp.Run},
		{analysisapp.Name, "127.0.0.1:8085", analysisapp.LoadConfig, analysisapp.Run},
		{scoringapp.Name, "127.0.0.1:8086", scoringapp.LoadConfig, scoringapp.Run},
		{recommendationapp.Name, "127.0.0.1:8087", recommendationapp.LoadConfig, recommendationapp.Run},
		{reportapp.Name, "127.0.0.1:8088", reportapp.LoadConfig, reportapp.Run},
	}
	type runningService struct {
		name   string
		url    string
		cancel context.CancelFunc
		stop   func()
	}
	var running []runningService
	for _, service := range services {
		cfg, err := service.load()
		if err != nil {
			t.Fatalf("%s config: %v", service.name, err)
		}
		if cfg.HTTPAddr != service.defaultAddr {
			t.Fatalf("%s address: got %s, want %s", service.name, cfg.HTTPAddr, service.defaultAddr)
		}
		cfg.HTTPAddr = "127.0.0.1:0"
		cfg.ShutdownTimeout = time.Second
		ready := make(chan startupEvent, 1)
		log := slog.New(slog.NewJSONHandler(startupWriter{ready: ready}, nil)).With("service", service.name)
		ctx, cancel := context.WithCancel(logger.With(context.Background(), log))
		done := make(chan error, 1)
		stop := sync.OnceFunc(func() {
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("%s shutdown: %v", service.name, err)
				}
			case <-time.After(5 * time.Second):
				t.Errorf("%s did not stop", service.name)
			}
		})
		t.Cleanup(stop)
		go func() {
			defer close(done)
			done <- service.run(ctx, cfg)
		}()
		select {
		case event := <-ready:
			if event.Service != service.name || event.Address == "" {
				t.Fatalf("invalid startup event for %s: %+v", service.name, event)
			}
			running = append(running, runningService{service.name, "http://" + event.Address, cancel, stop})
		case err := <-done:
			t.Fatalf("%s stopped during startup: %v", service.name, err)
		case <-time.After(5 * time.Second):
			t.Fatalf("%s did not start", service.name)
		}
	}

	// All nine applications run concurrently on independent real listeners.
	client := &http.Client{Timeout: 3 * time.Second}
	defer client.CloseIdleConnections()
	for _, service := range running {
		t.Run(service.name, func(t *testing.T) {
			response, err := client.Get(service.url + "/health")
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil || response.StatusCode != http.StatusOK {
				t.Fatalf("health status=%d, error=%v", response.StatusCode, err)
			}
			var health struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal(body, &health); err != nil || health.Status != "ok" {
				t.Fatalf("invalid health response: %s", body)
			}
			if response.Header.Get("X-Request-ID") == "" {
				t.Fatal("request logger did not add X-Request-ID")
			}
			for _, path := range []string{"/api/v1/analyses", "/api/v1/analyses/A123", "/api/v1/reports/R123"} {
				method := http.MethodGet
				if path == "/api/v1/analyses" {
					method = http.MethodPost
				}
				request, err := http.NewRequest(method, service.url+path, nil)
				if err != nil {
					t.Fatal(err)
				}
				response, err := client.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				io.Copy(io.Discard, response.Body)
				response.Body.Close()
				want := http.StatusNotFound
				if service.name == apiapp.Name {
					want = http.StatusNotImplemented
				}
				if response.StatusCode != want {
					t.Errorf("%s %s: got %d, want %d", method, path, response.StatusCode, want)
				}
			}
		})
	}

	for _, service := range running {
		service.cancel()
	}
	for _, service := range running {
		service.stop()
	}
}
