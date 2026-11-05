package app

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	GRPCAddr        string
	DatabaseURL     string
	DatabaseTimeout time.Duration
	ShutdownTimeout time.Duration
}

func LoadConfig() (Config, error) { return loadConfig(os.LookupEnv) }

func loadConfig(lookup func(string) (string, bool)) (Config, error) {
	value := func(key, fallback string) string {
		if raw, ok := lookup(key); ok {
			return strings.TrimSpace(raw)
		}
		return fallback
	}
	cfg := Config{GRPCAddr: value("REPORT_GRPC_ADDR", "127.0.0.1:9091"), DatabaseURL: value("REPORT_DATABASE_URL", "")}
	_, port, err := net.SplitHostPort(cfg.GRPCAddr)
	portNumber, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || portNumber < 1 || portNumber > 65535 {
		return Config{}, errors.New("REPORT_GRPC_ADDR must use host:port")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("REPORT_DATABASE_URL is required")
	}
	cfg.DatabaseTimeout, err = time.ParseDuration(value("REPORT_DATABASE_TIMEOUT", "5s"))
	if err != nil || cfg.DatabaseTimeout <= 0 {
		return Config{}, errors.New("REPORT_DATABASE_TIMEOUT must be positive")
	}
	cfg.ShutdownTimeout, err = time.ParseDuration(value("SHUTDOWN_TIMEOUT", "10s"))
	if err != nil || cfg.ShutdownTimeout <= 0 {
		return Config{}, errors.New("SHUTDOWN_TIMEOUT must be positive")
	}
	return cfg, nil
}
