package app

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL     string
	DatabaseTimeout time.Duration
	PollInterval    time.Duration
	AnalysisTimeout time.Duration
	GitTimeout      time.Duration
	GitMaxBytes     int64
	GitMaxPaths     int
	GitMaxCommits   int
	TempDirectory   string
}

func LoadConfig() (Config, error) { return loadConfig(os.LookupEnv) }

func loadConfig(lookup func(string) (string, bool)) (Config, error) {
	url, _ := lookup("ANALYZER_DATABASE_URL")
	if strings.TrimSpace(url) == "" {
		return Config{}, errors.New("ANALYZER_DATABASE_URL is required")
	}
	timeout := "5s"
	if value, ok := lookup("ANALYZER_DATABASE_TIMEOUT"); ok {
		timeout = value
	}
	duration, err := time.ParseDuration(timeout)
	if err != nil || duration <= 0 {
		return Config{}, errors.New("ANALYZER_DATABASE_TIMEOUT must be a positive duration")
	}
	cfg := Config{DatabaseURL: url, DatabaseTimeout: duration, PollInterval: time.Second, AnalysisTimeout: 320 * time.Second, GitTimeout: 60 * time.Second, GitMaxBytes: 100 << 20, GitMaxPaths: 50000, GitMaxCommits: 10000}
	for _, option := range []struct {
		key    string
		target *time.Duration
		max    time.Duration
	}{
		{"ANALYZER_POLL_INTERVAL", &cfg.PollInterval, time.Minute},
		{"ANALYZER_TIMEOUT", &cfg.AnalysisTimeout, 320 * time.Second},
		{"ANALYZER_GIT_TIMEOUT", &cfg.GitTimeout, 60 * time.Second},
	} {
		if value, ok := lookup(option.key); ok {
			parsed, err := time.ParseDuration(value)
			if err != nil || parsed <= 0 || parsed > option.max {
				return Config{}, fmt.Errorf("%s must be positive and at most %s", option.key, option.max)
			}
			*option.target = parsed
		}
	}
	for _, option := range []struct {
		key    string
		target *int
		max    int
	}{
		{"ANALYZER_GIT_MAX_PATHS", &cfg.GitMaxPaths, 50000},
		{"ANALYZER_GIT_MAX_COMMITS", &cfg.GitMaxCommits, 10000},
	} {
		if value, ok := lookup(option.key); ok {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 || parsed > option.max {
				return Config{}, fmt.Errorf("%s is outside the collector profile", option.key)
			}
			*option.target = parsed
		}
	}
	if value, ok := lookup("ANALYZER_GIT_MAX_BYTES"); ok {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed <= 0 || parsed > 100<<20 {
			return Config{}, errors.New("ANALYZER_GIT_MAX_BYTES must be in 1..104857600")
		}
		cfg.GitMaxBytes = parsed
	}
	cfg.TempDirectory, _ = lookup("ANALYZER_TEMP_DIR")
	return cfg, nil
}
