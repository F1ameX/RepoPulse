// Package repository persists Analyzer results in the shared PostgreSQL database.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

const analysisSelect = `SELECT a.id::text, a.submission_id::text, s.request_id::text,
	a.owner_id::text, p.kind, a.repository, a.assessment_at, a.window_from, a.window_until,
	a.versions, a.status, a.created_at, a.started_at, a.deadline_at, a.completed_at, a.report_id::text, a.stage, a.error
	FROM analyzer.analyses a
	JOIN analyzer.submissions s ON s.id = a.submission_id
	JOIN auth.principals p ON p.id = a.owner_id`

func scanAnalysis(row pgx.Row) (domain.Analysis, error) {
	var a domain.Analysis
	var repository, versions, failure []byte
	if err := row.Scan(&a.ID, &a.SubmissionID, &a.RequestID, &a.Owner.ID, &a.Owner.Kind, &repository,
		&a.AssessmentAt, &a.Window.From, &a.Window.Until, &versions, &a.Status, &a.CreatedAt,
		&a.StartedAt, &a.DeadlineAt, &a.CompletedAt, &a.ReportID, &a.Stage, &failure); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return a, domain.ErrNotFound
		}
		return a, fmt.Errorf("read analysis: %w", err)
	}
	if failure != nil {
		if err := json.Unmarshal(failure, &a.Error); err != nil {
			return a, fmt.Errorf("decode failure: %w", err)
		}
	}
	if err := json.Unmarshal(repository, &a.Repository); err != nil {
		return a, fmt.Errorf("decode repository: %w", err)
	}
	if err := json.Unmarshal(versions, &a.Versions); err != nil {
		return a, fmt.Errorf("decode versions: %w", err)
	}
	a.AssessmentAt, a.CreatedAt = domain.UTC(a.AssessmentAt), domain.UTC(a.CreatedAt)
	a.Window.From, a.Window.Until = domain.UTC(a.Window.From), domain.UTC(a.Window.Until)
	for _, at := range []**time.Time{&a.StartedAt, &a.DeadlineAt, &a.CompletedAt} {
		if *at != nil {
			normalized := domain.UTC(**at)
			*at = &normalized
		}
	}
	return a, nil
}

func (p *Postgres) Get(ctx context.Context, id string) (domain.Analysis, error) {
	return scanAnalysis(p.pool.QueryRow(ctx, analysisSelect+" WHERE a.id = $1", id))
}

func (p *Postgres) Create(ctx context.Context, a domain.Analysis) (domain.Analysis, error) {
	if err := a.Validate(); err != nil {
		return domain.Analysis{}, err
	}
	if a.Status != "queued" || a.StartedAt != nil || a.DeadlineAt != nil || a.CompletedAt != nil || a.ReportID != nil {
		return domain.Analysis{}, domain.ErrInvalidTransition
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.Analysis{}, fmt.Errorf("begin acceptance: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	// Confirm principal kind without creating or altering Auth-owned identities.
	var kind string
	if err := tx.QueryRow(ctx, "SELECT kind FROM auth.principals WHERE id = $1", a.Owner.ID).Scan(&kind); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Analysis{}, domain.ErrUnverifiedOwner
		}
		return domain.Analysis{}, fmt.Errorf("read principal: %w", err)
	}
	if kind != a.Owner.Kind {
		return domain.Analysis{}, domain.ErrUnverifiedOwner
	}
	tag, err := tx.Exec(ctx, `INSERT INTO analyzer.submissions
		(id, owner_id, request_id, canonical_url, assessment_at, state, analysis_id)
		VALUES ($1,$2,$3,$4,$5,'accepted',$6) ON CONFLICT (owner_id,request_id) DO NOTHING`,
		a.SubmissionID, a.Owner.ID, a.RequestID, a.Repository.CanonicalURL, a.AssessmentAt, a.ID)
	if err != nil {
		return domain.Analysis{}, fmt.Errorf("insert submission: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var originalURL string
		var analysisID *string
		if err := tx.QueryRow(ctx, `SELECT canonical_url, analysis_id::text FROM analyzer.submissions
			WHERE owner_id=$1 AND request_id=$2`, a.Owner.ID, a.RequestID).Scan(&originalURL, &analysisID); err != nil {
			return domain.Analysis{}, fmt.Errorf("read submission: %w", err)
		}
		if originalURL != a.Repository.CanonicalURL {
			return domain.Analysis{}, domain.ErrSubmissionChanged
		}
		if analysisID == nil {
			return domain.Analysis{}, domain.ErrSubmissionPending
		}
		return scanAnalysis(tx.QueryRow(ctx, analysisSelect+" WHERE a.id=$1", *analysisID))
	}
	repository, _ := json.Marshal(a.Repository)
	versions, _ := json.Marshal(a.Versions)
	_, err = tx.Exec(ctx, `INSERT INTO analyzer.analyses
		(id,submission_id,owner_id,canonical_url,repository,assessment_at,window_from,window_until,versions,status,created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'queued',$10)`,
		a.ID, a.SubmissionID, a.Owner.ID, a.Repository.CanonicalURL, repository, a.AssessmentAt,
		a.Window.From, a.Window.Until, versions, a.CreatedAt)
	if err != nil {
		return domain.Analysis{}, fmt.Errorf("insert analysis: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Analysis{}, fmt.Errorf("commit acceptance: %w", err)
	}
	return a, nil
}

func (p *Postgres) Start(ctx context.Context, id string, at, deadline time.Time) error {
	at, deadline = domain.UTC(at), domain.UTC(deadline)
	if !deadline.After(at) {
		return domain.ErrInvalidData
	}
	tag, err := p.pool.Exec(ctx, `UPDATE analyzer.analyses SET status='running', stage='collecting_git', started_at=$2, deadline_at=$3
		WHERE id=$1 AND status='queued' AND created_at <= $2`, id, domain.UTC(at), domain.UTC(deadline))
	if err != nil {
		return fmt.Errorf("start analysis: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrInvalidTransition
	}
	return nil
}

func (p *Postgres) Complete(ctx context.Context, result domain.Result) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin result: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	a, err := scanAnalysis(tx.QueryRow(ctx, analysisSelect+" WHERE a.id=$1 FOR UPDATE OF a", result.Snapshot.AnalysisID))
	if err != nil {
		return err
	}
	// Revalidate against the locked row, not the earlier worker copy.
	if err := result.Validate(a); err != nil {
		return err
	}
	snapshot, err := json.Marshal(result.Snapshot)
	if err != nil {
		return domain.ErrInvalidData
	}
	report, err := json.Marshal(result.Report)
	if err != nil {
		return domain.ErrInvalidData
	}
	_, err = tx.Exec(ctx, `INSERT INTO analyzer.snapshots (id,analysis_id,owner_id,payload,created_at)
		VALUES ($1,$2,$3,$4,$5)`, result.Snapshot.ID, a.ID, a.Owner.ID, snapshot, result.Report.CompletedAt)
	if err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}
	for _, d := range result.Snapshot.Dependencies {
		origins, _ := json.Marshal(d.Origins)
		_, err = tx.Exec(ctx, `INSERT INTO analyzer.snapshot_dependencies
			(snapshot_id,ecosystem,package,version,origins) VALUES ($1,$2,$3,$4,$5)`,
			result.Snapshot.ID, d.Ecosystem, d.Package, d.Version, origins)
		if err != nil {
			return fmt.Errorf("insert dependency: %w", err)
		}
	}
	for _, v := range result.Snapshot.Vulnerabilities {
		_, err = tx.Exec(ctx, `INSERT INTO analyzer.snapshot_vulnerabilities
			(snapshot_id,vulnerability_id,ecosystem,package,version,severity,obtained_at,evidence_url)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, result.Snapshot.ID, v.ID, v.Ecosystem, v.Package,
			v.Version, v.Severity, domain.UTC(v.ObtainedAt), v.EvidenceURL)
		if err != nil {
			return fmt.Errorf("insert vulnerability: %w", err)
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO report.reports
		(id,analysis_id,snapshot_id,owner_id,completed_at,overall_score,payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, result.Report.ID, a.ID, result.Snapshot.ID, a.Owner.ID,
		result.Report.CompletedAt, result.Report.OverallScore, report)
	if err != nil {
		return fmt.Errorf("insert report: %w", err)
	}
	tag, err := tx.Exec(ctx, `UPDATE analyzer.analyses SET status='completed', stage=NULL, completed_at=$2, report_id=$3
		WHERE id=$1 AND status='running'`, a.ID, result.Report.CompletedAt, result.Report.ID)
	if err != nil {
		return fmt.Errorf("complete analysis: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrInvalidTransition
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit result: %w", err)
	}
	return nil
}

// GetResult performs a single statement so a read never mixes result versions.
// It only reads persisted JSON; no external source is queried.
func (p *Postgres) GetResult(ctx context.Context, id string) (domain.Result, error) {
	var snapshotJSON, reportJSON []byte
	err := p.pool.QueryRow(ctx, `SELECT s.payload,r.payload FROM analyzer.analyses a
		JOIN analyzer.snapshots s ON s.analysis_id=a.id AND s.owner_id=a.owner_id
		JOIN report.reports r ON r.analysis_id=a.id AND r.snapshot_id=s.id AND r.owner_id=a.owner_id AND r.id=a.report_id
		WHERE a.id=$1 AND a.status='completed'`, id).Scan(&snapshotJSON, &reportJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Result{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Result{}, fmt.Errorf("read result: %w", err)
	}
	snapshot, err := domain.DecodeSnapshot(snapshotJSON)
	if err != nil {
		return domain.Result{}, err
	}
	var report domain.Report
	if err := json.Unmarshal(reportJSON, &report); err != nil {
		return domain.Result{}, fmt.Errorf("decode report: %w", err)
	}
	return domain.Result{Snapshot: snapshot, Report: report}, nil
}
