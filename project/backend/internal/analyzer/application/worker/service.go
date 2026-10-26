// Package worker owns the persistent queue and bounded Analyzer lifecycle.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
)

type Queue interface {
	Claim(context.Context, time.Time, time.Duration) (domain.Analysis, error)
	Advance(context.Context, string, string, time.Time) error
	Pin(context.Context, string, domain.RepositoryMetadata) error
	Fail(context.Context, string, time.Time, domain.AnalysisFailure) error
	Expire(context.Context, time.Time) error
}
type GitSession interface {
	Data() domain.GitData
	ReadBlob(context.Context, string) ([]byte, error)
	Close() error
}
type GitSource interface {
	Open(context.Context, domain.RepositoryMetadata) (GitSession, error)
}
type Publisher interface {
	Complete(context.Context, string, domain.SnapshotData, domain.ReportData) (domain.Result, error)
}
type Output struct {
	Snapshot domain.SnapshotData
	Report   domain.ReportData
}

// Processor is the extension point for #18479 and subsequent calculation and
// explanation tasks. It receives only a bounded object reader, not a checkout.
// It must observe ctx and report confirmed stages via advance; it must not run
// repository code or persist raw source contents.
type Processor interface {
	Process(ctx context.Context, analysis domain.Analysis, git GitSession, advance func(string) error) (Output, error)
}

// An optional explanation adapter receives only the remaining job budget.
// A failed attempt may retain a processor-supplied deterministic fallback.
type Explainer interface {
	Explain(context.Context, domain.Analysis, Output) (Output, error)
}

type Service struct {
	queue     Queue
	git       GitSource
	processor Processor
	publisher Publisher
	budget    time.Duration
}

func NewService(queue Queue, git GitSource, processor Processor, publisher Publisher, budget time.Duration) *Service {
	return &Service{queue: queue, git: git, processor: processor, publisher: publisher, budget: budget}
}

// Run executes one job at a time in this process. Multiple processes safely
// share Claim's SKIP LOCKED queue; reads of status never call this method.
func (s *Service) Run(ctx context.Context, poll time.Duration) error {
	if s.budget <= 0 || s.budget > 320*time.Second || poll <= 0 {
		return domain.ErrInvalidData
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
		worked, err := s.RunOnce(ctx)
		if err != nil && ctx.Err() == nil {
			logger.From(ctx).ErrorContext(ctx, "Analyzer iteration failed", "code", "ANALYSIS_FAILED")
		}
		wait := poll
		if worked && err == nil {
			wait = 0
		}
		timer.Reset(wait)
	}
}
func (s *Service) RunOnce(ctx context.Context) (bool, error) {
	if err := s.queue.Expire(ctx, domain.UTC(time.Now())); err != nil {
		return false, err
	}
	a, err := s.queue.Claim(ctx, domain.UTC(time.Now()), s.budget)
	if errors.Is(err, domain.ErrQueueEmpty) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	jobCtx, cancel := context.WithDeadline(ctx, *a.DeadlineAt)
	defer cancel()
	err = s.execute(jobCtx, a)
	if err == nil {
		return true, nil
	}
	code := "ANALYSIS_FAILED"
	var failed *jobError
	if errors.As(err, &failed) {
		code = failed.code
	}
	if errors.Is(jobCtx.Err(), context.DeadlineExceeded) {
		code = "BUDGET_EXCEEDED"
	}
	failure := domain.Failure(code, a.ID)
	// Persist cancellation/failure with a separate short budget. A failed commit
	// does not overwrite an already completed terminal result.
	finish, done := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer done()
	if persistErr := s.queue.Fail(finish, a.ID, domain.UTC(time.Now()), failure); persistErr != nil {
		return true, persistErr
	}
	logger.From(ctx).WarnContext(ctx, "Analysis failed", "analysis_id", a.ID, "code", failure.Code)
	return true, nil
}

type jobError struct{ code string }

func (e *jobError) Error() string { return e.code }
func (s *Service) execute(ctx context.Context, a domain.Analysis) (err error) {
	defer func() {
		if recover() != nil {
			err = &jobError{"ANALYSIS_FAILED"}
		}
	}()
	git, err := s.git.Open(ctx, a.Repository)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := git.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	data := git.Data()
	if err = s.queue.Pin(ctx, a.ID, data.Repository); err != nil {
		return err
	}
	a.Repository = data.Repository
	if s.processor == nil {
		return &jobError{"ANALYSIS_FAILED"}
	}
	// Core processing ends by 300s; explanation may use up to 20s within the
	// total 320s. The stage callback also bounds a shorter configured budget.
	coreUntil := a.StartedAt.Add(300 * time.Second)
	if a.DeadlineAt.Before(coreUntil) {
		coreUntil = *a.DeadlineAt
	}
	processCtx, processCancel := context.WithDeadline(ctx, coreUntil)
	defer processCancel()
	current := "collecting_git"
	advance := func(stage string) error {
		if domain.StageOrder(stage) == 0 || domain.StageOrder(stage) < domain.StageOrder(current) || stage == "saving" {
			return domain.ErrInvalidTransition
		}
		if domain.StageOrder(stage) > domain.StageOrder("calculating") {
			return domain.ErrInvalidTransition
		}

		if err := s.queue.Advance(ctx, a.ID, stage, time.Now()); err != nil {
			return err
		}
		current = stage
		return nil
	}
	result, err := s.processor.Process(processCtx, a, git, advance)
	if errors.Is(processCtx.Err(), context.DeadlineExceeded) {
		return &jobError{"BUDGET_EXCEEDED"}
	}
	if err != nil {
		return err
	}
	if err = processCtx.Err(); err != nil {
		return err
	}
	processCancel()
	if explainer, ok := s.processor.(Explainer); ok {
		if err = s.queue.Advance(ctx, a.ID, "explaining", time.Now()); err != nil {
			return err
		}
		// Leave a small part of the total deadline for publishing the ready
		// fallback, rather than spending the entire remaining time on AI.
		reserve := min(time.Second, s.budget/10)
		explanationUntil := minTime(time.Now().Add(20*time.Second), a.DeadlineAt.Add(-reserve))
		explanationCtx, explanationCancel := context.WithDeadline(ctx, explanationUntil)
		explained, explanationErr := explainer.Explain(explanationCtx, a, result)
		explanationCancel()
		if explanationErr == nil {
			result = explained
		} else if result.Report.Explanation.Mode != "deterministic_fallback" {
			return &jobError{"ANALYSIS_FAILED"}
		}
	}
	if !useful(result) {
		return &jobError{"ANALYSIS_FAILED"}
	}
	// Collector completeness cannot be replaced with a claim of full coverage.
	data = git.Data()
	result.Snapshot.Refs = data.Refs
	sources := append([]domain.SourceState{}, data.Sources...)
	collected := map[string]bool{}
	for _, source := range data.Sources {
		collected[source.Source+"/"+source.Scope] = true
	}
	for _, source := range result.Snapshot.Sources {
		if !collected[source.Source+"/"+source.Scope] {
			sources = append(sources, source)
		}
	}
	result.Snapshot.Sources = sources
	// Raw Git objects are deleted before publication is allowed to become terminal.
	if err = git.Close(); err != nil {
		return err
	}
	if err = s.queue.Advance(ctx, a.ID, "saving", time.Now()); err != nil {
		return err
	}
	if _, err = s.publisher.Complete(ctx, a.ID, result.Snapshot, result.Report); err != nil {
		return &jobError{"REPORT_SAVE_FAILED"}
	}
	return nil
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func useful(output Output) bool {
	if len(output.Report.Findings) > 0 {
		return true
	}
	for _, input := range output.Snapshot.Inputs {
		raw, _ := json.Marshal(input.RawValue)
		if (input.Status == "complete" || input.Status == "partial") && string(raw) != "null" {
			return true
		}
	}
	return false
}
