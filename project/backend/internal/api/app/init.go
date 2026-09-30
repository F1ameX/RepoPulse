package app

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	httpapi "github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/in/http"
	"github.com/F1ameX/RepoPulse/project/backend/libs/logger"
)

func newHTTPServer(ctx context.Context, cfg Config) *http.Server {
	return &http.Server{
		Handler:           httpapi.NewHandler(),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.From(ctx).Handler(), slog.LevelError),
		BaseContext: func(net.Listener) context.Context {
			// Keep the service logger and other values while letting Shutdown drain
			// active requests instead of cancelling them with the process signal.
			return context.WithoutCancel(ctx)
		},
	}
}
