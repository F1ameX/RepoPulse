package app

import (
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
	cfg, err := loadConfig(fromMap(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		HTTPAddr:          "127.0.0.1:8080",
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: time.Minute,
		ShutdownTimeout:    10 * time.Second,
		GitHubCheckTimeout: 25 * time.Second,
		AuthGRPCTarget:     "127.0.0.1:9090", AuthRequestTimeout: 3 * time.Second,
		PublicOrigin: "http://localhost:3000", CookieSecure: true,
	}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	cfg, err := loadConfig(fromMap(map[string]string{
		"HTTP_ADDR":                "[::1]:9000",
		"HTTP_READ_HEADER_TIMEOUT": "1s", "HTTP_READ_TIMEOUT": "2s",
		"HTTP_WRITE_TIMEOUT": "3s", "HTTP_IDLE_TIMEOUT": "4s", "SHUTDOWN_TIMEOUT": "500ms",
		"GITHUB_CHECK_TIMEOUT": "2s", "GITHUB_TOKEN": "service-test-token",
		"AUTH_GRPC_TARGET": "auth:9191", "AUTH_REQUEST_TIMEOUT": "500ms",
		"PUBLIC_ORIGIN": "https://repopulse.example", "COOKIE_SECURE": "false",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		HTTPAddr:          "[::1]:9000",
		ReadHeaderTimeout: time.Second, ReadTimeout: 2 * time.Second,
		WriteTimeout: 3 * time.Second, IdleTimeout: 4 * time.Second,
		ShutdownTimeout:    500 * time.Millisecond,
		GitHubCheckTimeout: 2 * time.Second, GitHubToken: "service-test-token",
		AuthGRPCTarget: "auth:9191", AuthRequestTimeout: 500 * time.Millisecond,
		PublicOrigin: "https://repopulse.example", CookieSecure: false,
	}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestInvalidEnvironmentFailsEarly(t *testing.T) {
	cases := []struct{ key, value string }{
		{"HTTP_ADDR", ""}, {"HTTP_ADDR", "8080"},
		{"HTTP_ADDR", "localhost:0"}, {"HTTP_ADDR", "localhost:65536"},
		{"HTTP_ADDR", "localhost:http"},
		{"AUTH_GRPC_TARGET", ""}, {"AUTH_GRPC_TARGET", ":9090"}, {"AUTH_GRPC_TARGET", "auth"}, {"AUTH_GRPC_TARGET", "auth:65536"},
		{"PUBLIC_ORIGIN", ""}, {"PUBLIC_ORIGIN", "null"}, {"PUBLIC_ORIGIN", "https://site/path"}, {"PUBLIC_ORIGIN", "https://user@site"},
		{"COOKIE_SECURE", ""}, {"COOKIE_SECURE", "invalid"},
	}
	for _, key := range []string{
		"HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT",
		"HTTP_IDLE_TIMEOUT", "SHUTDOWN_TIMEOUT",
		"GITHUB_CHECK_TIMEOUT", "AUTH_REQUEST_TIMEOUT",
	} {
		for _, value := range []string{"", "0s", "-1s", "30", "invalid"} {
			cases = append(cases, struct{ key, value string }{key, value})
		}
	}
	for _, tc := range cases {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			_, err := loadConfig(fromMap(map[string]string{tc.key: tc.value}))
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("expected error naming %s, got %v", tc.key, err)
			}
		})
	}
}

func TestGitHubBudgetFitsHTTPResponseTimeout(t *testing.T) {
	for _, values := range []map[string]string{
		{"GITHUB_CHECK_TIMEOUT": "26s"},
		{"AUTH_REQUEST_TIMEOUT": "5s"},
		{"GITHUB_CHECK_TIMEOUT": "25s", "HTTP_WRITE_TIMEOUT": "25s"},
	} {
		if _, err := loadConfig(fromMap(values)); err == nil {
			t.Fatal("invalid GitHub budget accepted")
		}
	}
}
