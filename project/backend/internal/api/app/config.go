package app

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
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
	AuthGRPCTarget     string
	AuthRequestTimeout time.Duration
	PublicOrigin       string
	CookieSecure       bool
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

	cfg.AuthGRPCTarget = value("AUTH_GRPC_TARGET", "127.0.0.1:9090")
	authHost, authPort, err := net.SplitHostPort(cfg.AuthGRPCTarget)
	authPortNumber, portErr := strconv.Atoi(authPort)
	if err != nil || authHost == "" || portErr != nil || authPortNumber < 1 || authPortNumber > 65535 {
		return Config{}, fmt.Errorf("AUTH_GRPC_TARGET must use host:port")
	}
	cfg.PublicOrigin = value("PUBLIC_ORIGIN", "http://localhost:3000")
	origin, err := url.Parse(cfg.PublicOrigin)
	if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" || origin.ForceQuery {
		return Config{}, fmt.Errorf("PUBLIC_ORIGIN must be an http(s) origin without path")
	}
	cfg.CookieSecure, err = strconv.ParseBool(value("COOKIE_SECURE", "true"))
	if err != nil {
		return Config{}, fmt.Errorf("COOKIE_SECURE must be true or false")
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
		{"GITHUB_CHECK_TIMEOUT", "25s", &cfg.GitHubCheckTimeout},
		{"AUTH_REQUEST_TIMEOUT", "3s", &cfg.AuthRequestTimeout},
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
	if cfg.AuthRequestTimeout >= cfg.WriteTimeout/2 || cfg.AuthRequestTimeout >= cfg.WriteTimeout-cfg.GitHubCheckTimeout {
		return Config{}, fmt.Errorf("AUTH_REQUEST_TIMEOUT must leave time within HTTP_WRITE_TIMEOUT for two Auth calls or an Auth call followed by GITHUB_CHECK_TIMEOUT")
	}
	return cfg, nil
}
