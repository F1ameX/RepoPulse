package app

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	grpcadapter "github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/in/grpc"
	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/out/repository"
	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/out/token"
	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/application/auth"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newGRPCServer registers AuthService and the standard health service.
// Guest scenarios use PostgreSQL; account registration/login/logout remain stubs.
func newGRPCServer(ctx context.Context, pool *pgxpool.Pool) (*grpc.Server, *health.Server) {
	log := logger.From(ctx)
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(
		// Handlers receive the service logger through the request context.
		withRequestLogger(log), grpcerr.UnaryServerInterceptor()))

	sessionsRepo := repository.NewSessions(pool)
	authv1.RegisterAuthServiceServer(server, grpcadapter.NewServer(auth.NewService(sessionsRepo,
		repository.NewUsers(pool),
		sessionsRepo,
		token.NewGenerator(),
		func() time.Time { return time.Now().UTC() },
	)))

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	return server, healthServer
}

func withRequestLogger(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		l := log
		if ids := metadata.ValueFromIncomingContext(ctx, grpcerr.RequestIDKey); len(ids) > 0 {
			l = l.With("request_id", ids[0])
		}
		return handler(logger.With(ctx, l), req)
	}
}
