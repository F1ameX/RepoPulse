// Package app wires the Report Service process.
package app

import (
	"context"
	"net"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
)

const Name = "report-service"

func Run(ctx context.Context) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	server, closeStorage, err := openServer(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeStorage()
	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return err
	}
	defer listener.Close()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	logger.From(ctx).InfoContext(ctx, "Report gRPC ready", "address", listener.Addr().String())
	select {
	case err := <-done:
		server.Stop()
		return err
	case <-ctx.Done():
	}
	stopped := make(chan struct{})
	go func() { server.GracefulStop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(cfg.ShutdownTimeout):
		server.Stop()
		<-stopped
	}
	if err := <-done; err != nil {
		return err
	}
	logger.From(ctx).InfoContext(ctx, "Service stopped")
	return nil
}
