// Package httpserver manages HTTP server lifecycle for RepoPulse processes.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/libs/logger"
)

func Run(ctx context.Context, server *http.Server, shutdownTimeout time.Duration) error {
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	return serve(ctx, listener, server, shutdownTimeout)
}

func serve(ctx context.Context, listener net.Listener, server *http.Server, shutdownTimeout time.Duration) error {
	defer server.Close()
	log := logger.From(ctx)

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	log.InfoContext(ctx, "HTTP server listening", "address", listener.Addr().String())

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		log.InfoContext(ctx, "HTTP server shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	log.InfoContext(ctx, "HTTP server stopped")
	return nil
}
