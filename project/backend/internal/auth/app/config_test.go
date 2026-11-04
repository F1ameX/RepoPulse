package app

import (
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/out/password"
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
		"empty address":      {"AUTH_GRPC_ADDR": ""},
		"no port":            {"AUTH_GRPC_ADDR": "127.0.0.1"},
		"port out of range":  {"AUTH_GRPC_ADDR": "127.0.0.1:70000"},
		"zero port":          {"AUTH_GRPC_ADDR": "127.0.0.1:0"},
		"empty timeout":      {"SHUTDOWN_TIMEOUT": ""},
		"bad timeout":        {"SHUTDOWN_TIMEOUT": "soon"},
		"negative timeout":   {"SHUTDOWN_TIMEOUT": "-1s"},
		"empty database url": {"AUTH_DATABASE_URL": "  "},
		"zero guest ttl":     {"AUTH_GUEST_SESSION_TTL": "0s"},
		"bad account ttl":    {"AUTH_ACCOUNT_SESSION_TTL": "week"},
		"empty account ttl":  {"AUTH_ACCOUNT_SESSION_TTL": ""},
		"memory below min":   {"AUTH_ARGON2_MEMORY_KIB": "16384"},
		"memory above max":   {"AUTH_ARGON2_MEMORY_KIB": "4194304"},
		"zero time":          {"AUTH_ARGON2_TIME": "0"},
		"negative time":      {"AUTH_ARGON2_TIME": "-1"},
		"threads overflow":   {"AUTH_ARGON2_THREADS": "300"},
		"zero key length":    {"AUTH_ARGON2_KEY_LEN": "0"},
		"short salt":         {"AUTH_ARGON2_SALT_LEN": "8"},
		"empty salt length":  {"AUTH_ARGON2_SALT_LEN": ""},
		"non-numeric memory": {"AUTH_ARGON2_MEMORY_KIB": "19MiB"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadConfig(lookupFrom(env)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestLoadConfigSessionAndHasherDefaults(t *testing.T) {
	cfg, err := loadConfig(lookupFrom(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GuestSessionTTL != 24*time.Hour || cfg.AccountSessionTTL != 168*time.Hour {
		t.Fatalf("session TTLs = %v / %v; FR-27 requires 24h and 7 days", cfg.GuestSessionTTL, cfg.AccountSessionTTL)
	}
	want := password.Params{HashMem: 19456, HashTime: 2, HashThreads: 1, KeyLen: 32, SaltLen: 16}
	if cfg.HasherConfig != want {
		t.Fatalf("hasher = %+v, want %+v", cfg.HasherConfig, want)
	}
	if _, err := password.NewArgon2id(cfg.HasherConfig); err != nil {
		t.Fatalf("default hasher parameters are invalid: %v", err)
	}
}

func TestLoadConfigSessionAndHasherOverrides(t *testing.T) {
	cfg, err := loadConfig(lookupFrom(map[string]string{
		"AUTH_GUEST_SESSION_TTL":   "2m",
		"AUTH_ACCOUNT_SESSION_TTL": "5m",
		"AUTH_ARGON2_MEMORY_KIB":   "65536",
		"AUTH_ARGON2_TIME":         "3",
		"AUTH_ARGON2_THREADS":      "4",
		"AUTH_ARGON2_KEY_LEN":      "64",
		"AUTH_ARGON2_SALT_LEN":     "32",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := password.Params{HashMem: 65536, HashTime: 3, HashThreads: 4, KeyLen: 64, SaltLen: 32}
	if cfg.GuestSessionTTL != 2*time.Minute || cfg.AccountSessionTTL != 5*time.Minute || cfg.HasherConfig != want {
		t.Fatalf("config = %+v", cfg)
	}
}
