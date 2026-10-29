package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/adapter/out/github"
	analyzerrepo "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/adapter/out/repository"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/analysis"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/repository"
	httpapi "github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/in/http"
	authclient "github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/out/auth"
	"github.com/F1ameX/RepoPulse/project/backend/internal/api/application/session"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func newSessions(cfg Config) (*session.Service, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(cfg.AuthGRPCTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return session.NewService(authclient.NewClient(conn, cfg.AuthRequestTimeout)), conn, nil
}

// newAPIHandler extends the shared Auth/session routes with Analyzer scenarios.
func newAPIHandler(ctx context.Context, cfg Config, settings AnalysisConfig, sessions *session.Service) (http.Handler, func(), error) {
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
	client := github.NewClient(nil, cfg.GitHubToken)
	scenarios := analysis.NewService(analyzerrepo.NewPostgres(pool), client, settings.Versions, cfg.GitHubCheckTimeout)
	handler := httpapi.NewHandler(repository.NewValidator(client, cfg.GitHubCheckTimeout), sessions, httpapi.Options{
		Sessions: sessions, CookieSecure: cfg.CookieSecure, PublicOrigin: cfg.PublicOrigin, Analyses: scenarios,
	})
	return handler, pool.Close, nil
}

func newHTTPServer(ctx context.Context, cfg Config, sessions ...*session.Service) *http.Server {
	var resolver httpapi.ContextResolver
	var service httpapi.Sessions
	if len(sessions) > 0 && sessions[0] != nil {
		resolver, service = sessions[0], sessions[0]
	}
	origin := cfg.PublicOrigin
	if origin == "" {
		origin = "http://localhost:3000"
	}
	validator := repository.NewValidator(github.NewClient(nil, cfg.GitHubToken), cfg.GitHubCheckTimeout)
	return &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewHandler(validator, resolver, httpapi.Options{Sessions: service, CookieSecure: cfg.CookieSecure, PublicOrigin: origin}),
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
