// Package command bootstraps service logging and signal handling.
package command

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/F1ameX/RepoPulse/project/backend/libs/logger"
)

// Execute returns the process exit code.
func Execute(name string, run func(context.Context) error) int {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", name)
	level, err := loadLogLevel(os.LookupEnv)
	if err != nil {
		log.Error("Invalid configuration", "error", err)
		return 1
	}
	log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})).
		With("service", name)
	slog.SetDefault(log)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx = logger.With(ctx, log)
	if err := run(ctx); err != nil {
		log.ErrorContext(ctx, "Service stopped", "error", err)
		return 1
	}
	return 0
}

func loadLogLevel(lookup func(string) (string, bool)) (slog.Level, error) {
	value, ok := lookup("LOG_LEVEL")
	if !ok {
		return slog.LevelInfo, nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL must be debug, info, warn or error")
	}
}
