// Package command bootstraps a service process with logging and signal handling.
package command

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/config"
	"github.com/F1ameX/RepoPulse/project/backend/libs/logger"
)

// Execute returns the exit code after configuration, startup and shutdown.
func Execute(name string, load func() (config.Config, error), run func(context.Context, config.Config) error) int {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", name)
	cfg, err := load()
	if err != nil {
		log.Error("Invalid configuration", "error", err)
		return 1
	}
	log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})).
		With("service", name)
	slog.SetDefault(log)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx = logger.With(ctx, log)
	if err := run(ctx, cfg); err != nil {
		log.ErrorContext(ctx, "Service stopped", "error", err)
		return 1
	}
	return 0
}
