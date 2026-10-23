package app

import (
	"errors"
	"os"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL     string
	DatabaseTimeout time.Duration
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
	return Config{DatabaseURL: url, DatabaseTimeout: duration}, nil
}
