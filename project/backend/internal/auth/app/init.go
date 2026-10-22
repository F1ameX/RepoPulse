package app

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	grpcadapter "github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/in/grpc"
	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/application/auth"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
)

// newGRPCServer registers AuthService and the standard health service.
// Repository implementations are not wired yet: the scenarios are stubs.
func newGRPCServer(ctx context.Context) (*grpc.Server, *health.Server) {
	log := logger.From(ctx)
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(
		// Handlers receive the service logger through the request context.
		func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(logger.With(ctx, log), req)
		},
		grpcerr.UnaryServerInterceptor(),
	))
	authv1.RegisterAuthServiceServer(server, grpcadapter.NewServer(auth.NewService(nil, nil)))
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	return server, healthServer
}
