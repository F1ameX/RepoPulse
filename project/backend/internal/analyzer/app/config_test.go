package app

import (
	"strings"
	"testing"
	"time"
)

func TestStorageConfig(t *testing.T) {
	for _, tc := range []struct {
		name    string
		env     map[string]string
		fail    bool
		timeout time.Duration
	}{
		{"missing", nil, true, 0},
		{"default", map[string]string{"ANALYZER_DATABASE_URL": "postgres://localhost/test"}, false, 5 * time.Second},
		{"override", map[string]string{"ANALYZER_DATABASE_URL": "postgres://localhost/test", "ANALYZER_DATABASE_TIMEOUT": "2s"}, false, 2 * time.Second},
		{"empty timeout", map[string]string{"ANALYZER_DATABASE_URL": "postgres://localhost/test", "ANALYZER_DATABASE_TIMEOUT": ""}, true, 0},
		{"negative", map[string]string{"ANALYZER_DATABASE_URL": "postgres://localhost/test", "ANALYZER_DATABASE_TIMEOUT": "-1s"}, true, 0},
		{"secret URL", map[string]string{"ANALYZER_DATABASE_URL": "postgres://user:do-not-log@host/db", "ANALYZER_DATABASE_TIMEOUT": "invalid"}, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := loadConfig(func(key string) (string, bool) { v, ok := tc.env[key]; return v, ok })
			if (err != nil) != tc.fail {
				t.Fatalf("got %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "do-not-log") {
				t.Fatal("credential leaked")
			}
			if err == nil && cfg.DatabaseTimeout != tc.timeout {
				t.Fatalf("got %+v", cfg)
			}
		})
	}
}

func TestWorkerProfileCannotBeExpandedByEnvironment(t *testing.T) {
	for key, value := range map[string]string{
		"ANALYZER_TIMEOUT": "321s", "ANALYZER_GIT_TIMEOUT": "61s",
		"ANALYZER_GIT_MAX_BYTES": "104857601", "ANALYZER_GIT_MAX_PATHS": "50001",
		"ANALYZER_GIT_MAX_COMMITS": "10001", "ANALYZER_POLL_INTERVAL": "0s",
	} {
		t.Run(key, func(t *testing.T) {
			env := map[string]string{"ANALYZER_DATABASE_URL": "postgres://localhost/test", key: value}
			if _, err := loadConfig(func(k string) (string, bool) { v, ok := env[k]; return v, ok }); err == nil {
				t.Fatal("collector budget expanded or disabled")
			}
		})
	}
}
