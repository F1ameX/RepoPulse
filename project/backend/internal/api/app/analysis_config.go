package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

// AnalysisConfig adds persistence and deployed versions to the shared API config.
// Auth, Origin and cookie settings remain in Config from the main branch.
type AnalysisConfig struct {
	DatabaseURL     string
	DatabaseTimeout time.Duration
	Versions        domain.Versions
}

func loadAnalysisConfig(lookup func(string) (string, bool)) (AnalysisConfig, error) {
	value := func(key, fallback string) string {
		if raw, ok := lookup(key); ok {
			return strings.TrimSpace(raw)
		}
		return fallback
	}
	cfg := AnalysisConfig{DatabaseURL: value("ANALYZER_DATABASE_URL", "")}
	if cfg.DatabaseURL == "" {
		return AnalysisConfig{}, fmt.Errorf("ANALYZER_DATABASE_URL is required")
	}
	var err error
	cfg.DatabaseTimeout, err = time.ParseDuration(value("ANALYZER_DATABASE_TIMEOUT", "5s"))
	if err != nil || cfg.DatabaseTimeout <= 0 {
		return AnalysisConfig{}, fmt.Errorf("ANALYZER_DATABASE_TIMEOUT must be a positive duration")
	}
	if json.Unmarshal([]byte(value("ANALYZER_VERSIONS", "")), &cfg.Versions) != nil || cfg.Versions.Validate() != nil {
		return AnalysisConfig{}, fmt.Errorf("ANALYZER_VERSIONS must contain all eight version fields as JSON")
	}
	return cfg, nil
}
