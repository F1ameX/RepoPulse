package app

import (
	"testing"
	"time"
)

const testDatabaseURL = "postgres://auth:secret@127.0.0.1:5432/repopulse?sslmode=disable"

// lookupFrom returns a lookup over env with AUTH_DATABASE_URL set unless env overrides it.
func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		if value, ok := env[key]; ok {
			return value, true
		}
		if key == "AUTH_DATABASE_URL" {
			return testDatabaseURL, true
		}
		return "", false
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(lookupFrom(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GRPCAddr != "127.0.0.1:9090" || cfg.ShutdownTimeout != 10*time.Second || cfg.DatabaseURL != testDatabaseURL {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	cfg, err := loadConfig(lookupFrom(map[string]string{
		"AUTH_GRPC_ADDR":    " 0.0.0.0:9191 ",
		"SHUTDOWN_TIMEOUT":  "3s",
		"AUTH_DATABASE_URL": " postgres://other@db:5432/repopulse ",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GRPCAddr != "0.0.0.0:9191" || cfg.ShutdownTimeout != 3*time.Second ||
		cfg.DatabaseURL != "postgres://other@db:5432/repopulse" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadConfigRequiresDatabaseURL(t *testing.T) {
	lookup := func(key string) (string, bool) { return "", false }
	if _, err := loadConfig(lookup); err == nil {
		t.Fatal("expected an error without AUTH_DATABASE_URL")
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"empty address":     {"AUTH_GRPC_ADDR": ""},
		"no port":           {"AUTH_GRPC_ADDR": "127.0.0.1"},
		"port out of range": {"AUTH_GRPC_ADDR": "127.0.0.1:70000"},
		"zero port":         {"AUTH_GRPC_ADDR": "127.0.0.1:0"},
		"empty timeout":     {"SHUTDOWN_TIMEOUT": ""},
		"bad timeout":       {"SHUTDOWN_TIMEOUT": "soon"},
		"negative timeout":  {"SHUTDOWN_TIMEOUT": "-1s"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadConfig(lookupFrom(env)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
