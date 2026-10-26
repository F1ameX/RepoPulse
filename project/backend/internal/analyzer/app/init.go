package app

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	gitreader "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/adapter/out/git"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/adapter/out/repository"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/storage"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
)

// openRepository checks the shared schema; Compose or the developer applies
// migrations before starting the service.
func openRepository(ctx context.Context, cfg Config) (*repository.Postgres, func(), error) {
	ctx, cancel := context.WithTimeout(ctx, cfg.DatabaseTimeout)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, errors.New("invalid analyzer database configuration")
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		// Do not return driver parse/connection errors which can contain a DSN.
		return nil, nil, errors.New("analyzer database is unavailable")
	}
	var migrated bool
	err = pool.QueryRow(ctx, `SELECT to_regclass('auth.principals') IS NOT NULL
		AND to_regclass('analyzer.submissions') IS NOT NULL
		AND to_regclass('analyzer.analyses') IS NOT NULL
		AND to_regclass('analyzer.snapshots') IS NOT NULL
		AND to_regclass('analyzer.snapshot_dependencies') IS NOT NULL
		AND to_regclass('analyzer.snapshot_vulnerabilities') IS NOT NULL
		AND to_regclass('report.reports') IS NOT NULL`).Scan(&migrated)
	if err != nil || !migrated {
		pool.Close()
		return nil, nil, errors.New("apply auth/analyzer/report migrations before opening analysis storage")
	}
	return repository.NewPostgres(pool), pool.Close, nil
}

func OpenStorage(ctx context.Context, cfg Config) (*storage.Service, func(), error) {
	repo, close, err := openRepository(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	return storage.NewService(repo), close, nil
}

func OpenWorker(ctx context.Context, cfg Config, processor worker.Processor) (*worker.Service, func(), error) {
	repo, close, err := openRepository(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	git, err := gitreader.NewReader(cfg.TempDirectory, gitreader.Limits{Timeout: cfg.GitTimeout, Bytes: cfg.GitMaxBytes, Paths: cfg.GitMaxPaths, Commits: cfg.GitMaxCommits})
	if err != nil {
		close()
		return nil, nil, err
	}
	return worker.NewService(repo, git, processor, storage.NewService(repo), cfg.AnalysisTimeout), close, nil
}
