package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func fromMap(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func TestDefaults(t *testing.T) {
	cfg, err := loadConfig("127.0.0.1:8080", fromMap(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		HTTPAddr: "127.0.0.1:8080", LogLevel: slog.LevelInfo,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: time.Minute,
		ShutdownTimeout: 10 * time.Second,
	}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	cfg, err := loadConfig("127.0.0.1:8080", fromMap(map[string]string{
		"HTTP_ADDR": "[::1]:9000", "LOG_LEVEL": " DEBUG ",
		"HTTP_READ_HEADER_TIMEOUT": "1s", "HTTP_READ_TIMEOUT": "2s",
		"HTTP_WRITE_TIMEOUT": "3s", "HTTP_IDLE_TIMEOUT": "4s", "SHUTDOWN_TIMEOUT": "500ms",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		HTTPAddr: "[::1]:9000", LogLevel: slog.LevelDebug,
		ReadHeaderTimeout: time.Second, ReadTimeout: 2 * time.Second,
		WriteTimeout: 3 * time.Second, IdleTimeout: 4 * time.Second,
		ShutdownTimeout: 500 * time.Millisecond,
	}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestInvalidEnvironmentFailsEarly(t *testing.T) {
	cases := []struct{ key, value string }{
		{"HTTP_ADDR", ""}, {"HTTP_ADDR", "8080"},
		{"HTTP_ADDR", "localhost:0"}, {"HTTP_ADDR", "localhost:65536"},
		{"HTTP_ADDR", "localhost:http"}, {"LOG_LEVEL", "trace"}, {"LOG_LEVEL", ""},
	}
	for _, key := range []string{
		"HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT",
		"HTTP_IDLE_TIMEOUT", "SHUTDOWN_TIMEOUT",
	} {
		for _, value := range []string{"", "0s", "-1s", "30", "invalid"} {
			cases = append(cases, struct{ key, value string }{key, value})
		}
	}
	for _, tc := range cases {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			_, err := loadConfig("127.0.0.1:8080", fromMap(map[string]string{tc.key: tc.value}))
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("expected error naming %s, got %v", tc.key, err)
			}
		})
	}
}
