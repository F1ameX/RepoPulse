package app

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr          string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// LoadConfig reads API settings from the process environment.
// Explicitly empty values are invalid.
func LoadConfig() (Config, error) {
	return loadConfig(os.LookupEnv)
}

func loadConfig(lookup func(string) (string, bool)) (Config, error) {
	value := func(key, fallback string) string {
		if v, ok := lookup(key); ok {
			return strings.TrimSpace(v)
		}
		return fallback
	}

	cfg := Config{HTTPAddr: value("HTTP_ADDR", "127.0.0.1:8080")}
	_, port, err := net.SplitHostPort(cfg.HTTPAddr)
	if err != nil {
		return Config{}, fmt.Errorf("HTTP_ADDR must use host:port: %w", err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return Config{}, fmt.Errorf("HTTP_ADDR port must be between 1 and 65535")
	}

	for _, setting := range []struct {
		key      string
		fallback string
		target   *time.Duration
	}{
		{"HTTP_READ_HEADER_TIMEOUT", "5s", &cfg.ReadHeaderTimeout},
		{"HTTP_READ_TIMEOUT", "15s", &cfg.ReadTimeout},
		{"HTTP_WRITE_TIMEOUT", "30s", &cfg.WriteTimeout},
		{"HTTP_IDLE_TIMEOUT", "60s", &cfg.IdleTimeout},
		{"SHUTDOWN_TIMEOUT", "10s", &cfg.ShutdownTimeout},
	} {
		duration, err := time.ParseDuration(value(setting.key, setting.fallback))
		if err != nil || duration <= 0 {
			return Config{}, fmt.Errorf("%s must be a positive duration (for example, 5s)", setting.key)
		}
		*setting.target = duration
	}
	return cfg, nil
}
