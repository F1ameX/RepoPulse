package app

import (
	"testing"
	"time"
)

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(lookupFrom(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GRPCAddr != "127.0.0.1:9090" || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	cfg, err := loadConfig(lookupFrom(map[string]string{
		"AUTH_GRPC_ADDR":   " 0.0.0.0:9191 ",
		"SHUTDOWN_TIMEOUT": "3s",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GRPCAddr != "0.0.0.0:9191" || cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("config = %+v", cfg)
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
