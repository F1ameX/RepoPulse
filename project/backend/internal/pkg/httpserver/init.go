package httpserver

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/config"
	"github.com/F1ameX/RepoPulse/project/backend/libs/logger"
)

func New(ctx context.Context, cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
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
