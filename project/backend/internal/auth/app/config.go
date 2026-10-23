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
	GRPCAddr        string
	ShutdownTimeout time.Duration
	DatabaseURL     string
}

// LoadConfig reads Auth settings from the process environment.
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

	mandatoryValue := func(key string) (string, bool) {
		if v, ok := lookup(key); ok {
			return strings.TrimSpace(v), true
		}
		return "", false
	}

	cfg := Config{GRPCAddr: value("AUTH_GRPC_ADDR", "127.0.0.1:9090")}
	_, port, err := net.SplitHostPort(cfg.GRPCAddr)
	if err != nil {
		return Config{}, fmt.Errorf("AUTH_GRPC_ADDR must use host:port: %w", err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return Config{}, fmt.Errorf("AUTH_GRPC_ADDR port must be between 1 and 65535")
	}

	timeout, err := time.ParseDuration(value("SHUTDOWN_TIMEOUT", "10s"))
	if err != nil || timeout <= 0 {
		return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be a positive duration (for example, 5s)")
	}
	cfg.ShutdownTimeout = timeout

	databaseURL, ok := mandatoryValue("AUTH_DATABASE_URL")
	if !ok {
		return Config{}, fmt.Errorf("AUTH_DATABASE_URL must be presented")
	}
	cfg.DatabaseURL = databaseURL
	return cfg, nil
}
