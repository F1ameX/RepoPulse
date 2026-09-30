// Package app wires the API Service / BFF process.
package app

import (
	"context"

	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/httpserver"
)

const Name = "api-service"

func Run(ctx context.Context, cfg Config) error {
	return httpserver.Run(ctx, newHTTPServer(ctx, cfg), cfg.ShutdownTimeout)
}
