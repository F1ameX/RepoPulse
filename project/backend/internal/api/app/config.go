package app

import "github.com/F1ameX/RepoPulse/project/backend/internal/pkg/config"

// Config is the process configuration; service-specific settings can be added here.
type Config = config.Config

func LoadConfig() (Config, error) {
	return config.Load("127.0.0.1:8080")
}
