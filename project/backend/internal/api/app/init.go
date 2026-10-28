package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/adapter/out/github"
	analyzerrepo "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/adapter/out/repository"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/analysis"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/repository"
	httpapi "github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/in/http"
	authclient "github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/out/auth"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
)

func newAPIHandler(ctx context.Context, cfg Config, settings AnalysisConfig) (http.Handler, func(), error) {
	databaseCtx, cancel := context.WithTimeout(ctx, settings.DatabaseTimeout)
	defer cancel()
	pool, err := pgxpool.New(databaseCtx, settings.DatabaseURL)
	if err != nil {
		return nil, nil, errors.New("invalid analyzer database configuration")
	}
	if err := pool.Ping(databaseCtx); err != nil {
		pool.Close()
		return nil, nil, errors.New("analyzer database is unavailable")
	}
	var migrated bool
	err = pool.QueryRow(databaseCtx, `SELECT to_regclass('auth.principals') IS NOT NULL
		AND to_regclass('analyzer.submissions') IS NOT NULL AND to_regclass('analyzer.analyses') IS NOT NULL`).Scan(&migrated)
	if err != nil || !migrated {
		pool.Close()
		return nil, nil, errors.New("apply auth/analyzer migrations before starting the analysis API")
	}
	connection, err := grpc.NewClient(settings.AuthTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		pool.Close()
		return nil, nil, errors.New("invalid Auth gRPC configuration")
	}
	closeDependencies := func() {
		connection.Close()
		pool.Close()
	}
	client := github.NewClient(nil, cfg.GitHubToken)
	resolver := authclient.NewResolver(authv1.NewAuthServiceClient(connection), settings.AuthTimeout)
	scenarios := analysis.NewService(analyzerrepo.NewPostgres(pool), client, settings.Versions, cfg.GitHubCheckTimeout)
	handler := httpapi.NewAnalysisHandler(repository.NewValidator(client, cfg.GitHubCheckTimeout), resolver, scenarios, settings.AllowedOrigin)
	return handler, closeDependencies, nil
}

func newHTTPServer(ctx context.Context, cfg Config) *http.Server {
	validator := repository.NewValidator(github.NewClient(nil, cfg.GitHubToken), cfg.GitHubCheckTimeout)
	return &http.Server{
		Addr: cfg.HTTPAddr,
		// Run installs the production handler from newAPIHandler. Keeping this
		// isolated handler also allows transport lifecycle tests without a DB.
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
