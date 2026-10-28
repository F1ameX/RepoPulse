package app

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type Config struct {
	HTTPAddr           string
	ReadHeaderTimeout  time.Duration
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	ShutdownTimeout    time.Duration
	GitHubCheckTimeout time.Duration
	GitHubToken        string
}

type AnalysisConfig struct {
	DatabaseURL     string
	DatabaseTimeout time.Duration
	AuthTarget      string
	AuthTimeout     time.Duration
	AllowedOrigin   string
	Versions        domain.Versions
}

func loadAnalysisConfig(lookup func(string) (string, bool)) (AnalysisConfig, error) {
	value := func(key, fallback string) string {
		if raw, ok := lookup(key); ok {
			return strings.TrimSpace(raw)
		}
		return fallback
	}
	cfg := AnalysisConfig{
		DatabaseURL:   value("ANALYZER_DATABASE_URL", ""),
		AuthTarget:    value("AUTH_GRPC_TARGET", "127.0.0.1:9090"),
		AllowedOrigin: value("API_ALLOWED_ORIGIN", "http://localhost:5173"),
	}
	if cfg.DatabaseURL == "" {
		return AnalysisConfig{}, fmt.Errorf("ANALYZER_DATABASE_URL is required")
	}
	host, port, err := net.SplitHostPort(cfg.AuthTarget)
	portNumber, portErr := strconv.Atoi(port)
	if err != nil || host == "" || portErr != nil || portNumber < 1 || portNumber > 65535 {
		return AnalysisConfig{}, fmt.Errorf("AUTH_GRPC_TARGET must use host:port")
	}
	origin, err := url.Parse(cfg.AllowedOrigin)
	if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Host == "" ||
		origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.ForceQuery ||
		origin.Fragment != "" || strings.Contains(cfg.AllowedOrigin, "#") {
		return AnalysisConfig{}, fmt.Errorf("API_ALLOWED_ORIGIN must be an HTTP(S) origin without a path")
	}
	for _, setting := range []struct {
		key, fallback string
		target        *time.Duration
	}{
		{"ANALYZER_DATABASE_TIMEOUT", "5s", &cfg.DatabaseTimeout},
		{"AUTH_RPC_TIMEOUT", "3s", &cfg.AuthTimeout},
	} {
		parsed, err := time.ParseDuration(value(setting.key, setting.fallback))
		if err != nil || parsed <= 0 {
			return AnalysisConfig{}, fmt.Errorf("%s must be a positive duration", setting.key)
		}
		*setting.target = parsed
	}
	if json.Unmarshal([]byte(value("ANALYZER_VERSIONS", "")), &cfg.Versions) != nil || cfg.Versions.Validate() != nil {
		return AnalysisConfig{}, fmt.Errorf("ANALYZER_VERSIONS must contain all eight version fields as JSON")
	}
	return cfg, nil
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

	cfg := Config{HTTPAddr: value("HTTP_ADDR", "127.0.0.1:8080"), GitHubToken: value("GITHUB_TOKEN", "")}
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
		{"HTTP_WRITE_TIMEOUT", "35s", &cfg.WriteTimeout},
		{"HTTP_IDLE_TIMEOUT", "60s", &cfg.IdleTimeout},
		{"SHUTDOWN_TIMEOUT", "10s", &cfg.ShutdownTimeout},
		{"GITHUB_CHECK_TIMEOUT", "25s", &cfg.GitHubCheckTimeout},
	} {
		duration, err := time.ParseDuration(value(setting.key, setting.fallback))
		if err != nil || duration <= 0 {
			return Config{}, fmt.Errorf("%s must be a positive duration (for example, 5s)", setting.key)
		}
		*setting.target = duration
	}
	if cfg.GitHubCheckTimeout > 25*time.Second || cfg.GitHubCheckTimeout >= cfg.WriteTimeout {
		return Config{}, fmt.Errorf("GITHUB_CHECK_TIMEOUT must be at most 25s and less than HTTP_WRITE_TIMEOUT")
	}
	return cfg, nil
}
