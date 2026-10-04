// Package app wires the GitHub API Service process.
package app

import (
	"context"

	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
)

const Name = "github-api-service"

// Run currently initializes only logging and waits for shutdown.
func Run(ctx context.Context) error {
	logger.From(ctx).InfoContext(ctx, "Service scaffold started")
	<-ctx.Done()
	logger.From(ctx).InfoContext(ctx, "Service stopped")
	return nil
}
