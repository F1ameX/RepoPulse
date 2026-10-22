// Package app wires the Analyzer Service process.
package app

import (
	"context"

	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
)

const Name = "analyzer-service"

// Run opens Analyzer storage. Worker scheduling and incoming operations are
// connected by subsequent tasks; migrations are never run at process startup.
func Run(ctx context.Context) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	_, closeStorage, err := OpenStorage(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeStorage()
	logger.From(ctx).InfoContext(ctx, "Analysis storage ready")
	<-ctx.Done()
	logger.From(ctx).InfoContext(ctx, "Service stopped")
	return nil
}
