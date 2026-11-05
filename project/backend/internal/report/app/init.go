package app

import (
	"context"
	"errors"

	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
	incoming "github.com/F1ameX/RepoPulse/project/backend/internal/report/adapter/in/grpc"
	"github.com/F1ameX/RepoPulse/project/backend/internal/report/adapter/out/repository"
	"github.com/F1ameX/RepoPulse/project/backend/internal/report/application/report"
	wire "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/report/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func openServer(ctx context.Context, cfg Config) (*grpc.Server, func(), error) {
	databaseCtx, cancel := context.WithTimeout(ctx, cfg.DatabaseTimeout)
	defer cancel()
	pool, err := pgxpool.New(databaseCtx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, errors.New("invalid report database configuration")
	}
	if err := pool.Ping(databaseCtx); err != nil {
		pool.Close()
		return nil, nil, errors.New("report database unavailable")
	}
	var migrated bool
	if err := pool.QueryRow(databaseCtx, `SELECT to_regclass('report.reports') IS NOT NULL`).Scan(&migrated); err != nil || !migrated {
		pool.Close()
		return nil, nil, errors.New("apply report migrations before starting Report Service")
	}
	log := logger.From(ctx)
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(
		func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
			bounded, cancel := context.WithTimeout(logger.With(ctx, log), cfg.DatabaseTimeout)
			defer cancel()
			return next(bounded, req)
		}, grpcerr.UnaryServerInterceptor()))
	wire.RegisterReportServiceServer(server, incoming.NewServer(report.NewService(repository.NewPostgres(pool))))
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(wire.ReportService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	return server, pool.Close, nil
}
