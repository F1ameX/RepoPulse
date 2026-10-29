package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	"github.com/jackc/pgx/v5"
)

// Claim commits the ownership transition before any Git I/O. Concurrent workers
// skip locked rows rather than executing the same queued job.
func (p *Postgres) Claim(ctx context.Context, at time.Time, budget time.Duration) (domain.Analysis, error) {
	if budget <= 0 || budget > 320*time.Second {
		return domain.Analysis{}, domain.ErrInvalidData
	}
	at = domain.UTC(at)
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.Analysis{}, err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	var id string
	err = tx.QueryRow(ctx, `SELECT id::text FROM analyzer.analyses WHERE status='queued' AND created_at<=$1
  ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1`, at).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Analysis{}, domain.ErrQueueEmpty
	}
	if err != nil {
		return domain.Analysis{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE analyzer.analyses SET status='running',stage='collecting_git',started_at=$2,deadline_at=$3 WHERE id=$1`, id, at, at.Add(budget))
	if err != nil {
		return domain.Analysis{}, err
	}
	a, err := scanAnalysis(tx.QueryRow(ctx, analysisSelect+" WHERE a.id=$1", id))
	if err != nil {
		return a, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Analysis{}, err
	}
	return a, nil
}

func (p *Postgres) Advance(ctx context.Context, id, stage string, at time.Time) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	a, err := scanAnalysis(tx.QueryRow(ctx, analysisSelect+" WHERE a.id=$1 FOR UPDATE OF a", id))
	if err != nil {
		return err
	}
	if !domain.CanAdvance(a, stage, at) {
		return domain.ErrInvalidTransition
	}
	if _, err = tx.Exec(ctx, "UPDATE analyzer.analyses SET stage=$2 WHERE id=$1", id, stage); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Pin fills only previously unknown Git metadata. Known accepted values, owner,
// window, assessment time and versions are immutable.
func (p *Postgres) Pin(ctx context.Context, id string, metadata domain.RepositoryMetadata) error {
	if err := metadata.Validate(); err != nil {
		return err
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	a, err := scanAnalysis(tx.QueryRow(ctx, analysisSelect+" WHERE a.id=$1 FOR UPDATE OF a", id))
	if err != nil {
		return err
	}
	if a.Status != "running" {
		return domain.ErrInvalidTransition
	}
	original := a.Repository
	if original.CommitSHA != nil && (metadata.CommitSHA == nil || *metadata.CommitSHA != *original.CommitSHA) {
		return domain.ErrInvalidData
	}
	if original.DefaultBranch != nil && (metadata.DefaultBranch == nil || *metadata.DefaultBranch != *original.DefaultBranch) {
		return domain.ErrInvalidData
	}
	original.CommitSHA, original.DefaultBranch = metadata.CommitSHA, metadata.DefaultBranch
	left, _ := json.Marshal(original)
	right, _ := json.Marshal(metadata)
	if string(left) != string(right) {
		return domain.ErrInvalidData
	}
	encoded, _ := json.Marshal(metadata)
	if _, err = tx.Exec(ctx, "UPDATE analyzer.analyses SET repository=$2 WHERE id=$1", id, encoded); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Postgres) Fail(ctx context.Context, id string, at time.Time, failure domain.AnalysisFailure) error {
	safe := domain.Failure(failure.Code, id)
	encoded, _ := json.Marshal(safe)
	tag, err := p.pool.Exec(ctx, `UPDATE analyzer.analyses SET status='failed',stage=NULL,completed_at=$2,error=$3
  WHERE id=$1 AND status IN ('queued','running')`, id, domain.UTC(at), encoded)
	if err != nil {
		return fmt.Errorf("fail analysis: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrInvalidTransition
	}
	return nil
}

// A crashed/cancelled worker is not retried under the same analysis. Its bounded
// lease eventually becomes failed, preserving terminality and avoiding duplicate work.
func (p *Postgres) Expire(ctx context.Context, at time.Time) error {
	failure := domain.Failure("BUDGET_EXCEEDED", "")
	encoded, _ := json.Marshal(failure)
	_, err := p.pool.Exec(ctx, `UPDATE analyzer.analyses SET status='failed',stage=NULL,completed_at=$1,
 error=jsonb_set($2::jsonb,'{trace_id}',to_jsonb(id::text))
 WHERE status='running' AND deadline_at<=$1`, domain.UTC(at), encoded)
	return err
}
