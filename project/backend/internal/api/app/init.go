package app

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/adapter/out/github"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/repository"
	httpapi "github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/in/http"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
)

func newHTTPServer(ctx context.Context, cfg Config) *http.Server {
	validator := repository.NewValidator(github.NewClient(nil, cfg.GitHubToken), cfg.GitHubCheckTimeout)
	return &http.Server{
		Addr: cfg.HTTPAddr,
		// Auth.ResolveContext is not connected yet. A nil resolver keeps the
		// contract's session requirement instead of enabling anonymous access.
		Handler:           httpapi.NewHandler(validator, nil),
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
