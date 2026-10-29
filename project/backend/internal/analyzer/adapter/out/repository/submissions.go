package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func (p *Postgres) ReserveSubmission(ctx context.Context, requested domain.Submission) (domain.Submission, bool, error) {
	if requested.Owner.Validate() != nil || !domain.ValidID(requested.ID) || !domain.ValidID(requested.RequestID) {
		return domain.Submission{}, false, domain.ErrInvalidData
	}
	tag, err := p.pool.Exec(ctx, `INSERT INTO analyzer.submissions
		(id,owner_id,request_id,canonical_url,assessment_at,state)
		VALUES ($1,$2,$3,$4,$5,'pending') ON CONFLICT (owner_id,request_id) DO NOTHING`,
		requested.ID, requested.Owner.ID, requested.RequestID, requested.CanonicalURL, requested.AssessmentAt)
	if err != nil {
		return domain.Submission{}, false, fmt.Errorf("reserve submission: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return requested, true, nil
	}
	var stored domain.Submission
	var failure []byte
	stored.Owner = requested.Owner
	err = p.pool.QueryRow(ctx, `SELECT id::text,request_id::text,canonical_url,assessment_at,state,analysis_id::text,error
		FROM analyzer.submissions WHERE owner_id=$1 AND request_id=$2`, requested.Owner.ID, requested.RequestID).
		Scan(&stored.ID, &stored.RequestID, &stored.CanonicalURL, &stored.AssessmentAt, &stored.State, &stored.AnalysisID, &failure)
	if err != nil {
		return domain.Submission{}, false, fmt.Errorf("read reserved submission: %w", err)
	}
	if len(failure) > 0 && json.Unmarshal(failure, &stored.Failure) != nil {
		return domain.Submission{}, false, domain.ErrInvalidData
	}
	stored.AssessmentAt = domain.UTC(stored.AssessmentAt)
	return stored, false, nil
}

func (p *Postgres) AcceptSubmission(ctx context.Context, a domain.Analysis) (domain.Analysis, error) {
	if err := a.Validate(); err != nil {
		return domain.Analysis{}, err
	}
	if a.Status != "queued" || a.Stage != nil || a.Error != nil || a.ReportID != nil ||
		a.StartedAt != nil || a.CompletedAt != nil || a.DeadlineAt != nil {
		return domain.Analysis{}, domain.ErrInvalidTransition
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.Analysis{}, fmt.Errorf("begin submission acceptance: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	var state, ownerID, requestID string
	var analysisID *string
	err = tx.QueryRow(ctx, `SELECT state,owner_id::text,request_id::text,analysis_id::text
		FROM analyzer.submissions WHERE id=$1 FOR UPDATE`, a.SubmissionID).
		Scan(&state, &ownerID, &requestID, &analysisID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Analysis{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Analysis{}, fmt.Errorf("lock submission: %w", err)
	}
	if ownerID != a.Owner.ID || requestID != a.RequestID {
		return domain.Analysis{}, domain.ErrUnverifiedOwner
	}
	if analysisID != nil {
		return scanAnalysis(tx.QueryRow(ctx, analysisSelect+" WHERE a.id=$1", *analysisID))
	}
	if state != "pending" {
		return domain.Analysis{}, domain.ErrInvalidTransition
	}
	repository, _ := json.Marshal(a.Repository)
	versions, _ := json.Marshal(a.Versions)
	_, err = tx.Exec(ctx, `INSERT INTO analyzer.analyses
		(id,submission_id,owner_id,canonical_url,repository,assessment_at,window_from,window_until,versions,status,created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'queued',$10)`,
		a.ID, a.SubmissionID, a.Owner.ID, a.Repository.CanonicalURL, repository, a.AssessmentAt,
		a.Window.From, a.Window.Until, versions, a.CreatedAt)
	if err != nil {
		return domain.Analysis{}, fmt.Errorf("insert accepted analysis: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE analyzer.submissions SET state='accepted',analysis_id=$2 WHERE id=$1`, a.SubmissionID, a.ID)
	if err != nil {
		return domain.Analysis{}, fmt.Errorf("accept submission: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Analysis{}, fmt.Errorf("commit accepted submission: %w", err)
	}
	return a, nil
}

func (p *Postgres) RejectSubmission(ctx context.Context, id string, failure domain.SubmissionFailure) error {
	data, _ := json.Marshal(failure)
	tag, err := p.pool.Exec(ctx, `UPDATE analyzer.submissions SET state='rejected',error=$2
		WHERE id=$1 AND state='pending' AND analysis_id IS NULL`, id, data)
	if err != nil {
		return fmt.Errorf("reject submission: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrInvalidTransition
	}
	return nil
}
