// Package app wires the API process and owns its HTTP server lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/F1ameX/RepoPulse/project/backend/libs/logger"
)

func Run(ctx context.Context, cfg Config) error {
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	return serve(ctx, listener, cfg)
}

func serve(ctx context.Context, listener net.Listener, cfg Config) error {
	server := newHTTPServer(ctx, cfg)
	defer server.Close()
	log := logger.From(ctx)

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	log.InfoContext(ctx, "API listening", "address", listener.Addr().String())

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		log.InfoContext(ctx, "API shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	log.InfoContext(ctx, "API stopped")
	return nil
}
