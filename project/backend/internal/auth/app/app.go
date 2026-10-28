// Package app wires the Auth Service process.
package app

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
	"github.com/jackc/pgx/v5/pgxpool"
)

const Name = "auth-service"

func Run(ctx context.Context) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	options := domain.Options{
		GuestSessionTTL: cfg.GuestSessionTTL,
		AccountSessionTTL: cfg.AccountSessionTTL,
	}
	server, healthServer := newGRPCServer(ctx, pool, options)
	return serve(ctx, listener, server, healthServer, cfg.ShutdownTimeout)
}

func serve(ctx context.Context, listener net.Listener, server *grpc.Server, healthServer *health.Server, shutdownTimeout time.Duration) error {
	log := logger.From(ctx)

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	setServing(healthServer, healthpb.HealthCheckResponse_SERVING)
	log.InfoContext(ctx, "gRPC server listening", "address", listener.Addr().String())

	select {
	case err := <-serveErr:
		server.Stop()
		return fmt.Errorf("serve gRPC: %w", err)
	case <-ctx.Done():
		log.InfoContext(ctx, "gRPC server shutting down")
	}

	setServing(healthServer, healthpb.HealthCheckResponse_NOT_SERVING)
	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(shutdownTimeout):
		log.WarnContext(ctx, "gRPC graceful shutdown timed out")
		server.Stop()
		<-stopped
	}
	if err := <-serveErr; err != nil {
		return fmt.Errorf("serve gRPC: %w", err)
	}
	log.InfoContext(ctx, "gRPC server stopped")
	return nil
}

func setServing(healthServer *health.Server, status healthpb.HealthCheckResponse_ServingStatus) {
	healthServer.SetServingStatus("", status)
	healthServer.SetServingStatus(authv1.AuthService_ServiceDesc.ServiceName, status)
}
