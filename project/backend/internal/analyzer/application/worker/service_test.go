package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type memoryQueue struct {
	mu       sync.Mutex
	analysis domain.Analysis
	claims   int
	stages   []string
}

func (q *memoryQueue) Expire(context.Context, time.Time) error { return nil }
func (q *memoryQueue) Claim(_ context.Context, at time.Time, budget time.Duration) (domain.Analysis, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.analysis.Status != "queued" {
		return domain.Analysis{}, domain.ErrQueueEmpty
	}
	q.claims++
	q.analysis.Status = "running"
	q.analysis.StartedAt = &at
	deadline := at.Add(budget)
	q.analysis.DeadlineAt = &deadline
	return q.analysis, nil
}
func (q *memoryQueue) Pin(_ context.Context, _ string, metadata domain.RepositoryMetadata) error {
	q.analysis.Repository = metadata
	return nil
}
func (q *memoryQueue) Advance(_ context.Context, _, stage string, now time.Time) error {
	if !domain.CanAdvance(q.analysis, stage, now) {
		return domain.ErrInvalidTransition
	}
	q.stages = append(q.stages, stage)
	q.analysis.Stage = &stage
	return nil
}
func (q *memoryQueue) Fail(_ context.Context, _ string, at time.Time, f domain.AnalysisFailure) error {
	if q.analysis.Status == "completed" || q.analysis.Status == "failed" {
		return domain.ErrInvalidTransition
	}
	q.analysis.Status = "failed"
	q.analysis.Error = &f
	q.analysis.CompletedAt = &at
	return nil
}

type fakeSession struct {
	data     domain.GitData
	closed   bool
	closeErr error
}

func (s *fakeSession) Data() domain.GitData                             { return s.data }
func (s *fakeSession) ReadBlob(context.Context, string) ([]byte, error) { return nil, nil }
func (s *fakeSession) Close() error                                     { s.closed = true; return s.closeErr }

type source struct {
	s     *fakeSession
	opens int
}

func (s *source) Open(context.Context, domain.RepositoryMetadata) (GitSession, error) {
	s.opens++
	return s.s, nil
}

type processorFunc func(context.Context, domain.Analysis, GitSession, func(string) error) (Output, error)

func (p processorFunc) Process(ctx context.Context, a domain.Analysis, g GitSession, advance func(string) error) (Output, error) {
	return p(ctx, a, g, advance)
}

type publisher struct {
	queue   *memoryQueue
	session *fakeSession
	err     error
	calls   int
	output  domain.SnapshotData
}

func (p *publisher) Complete(_ context.Context, _ string, data domain.SnapshotData, _ domain.ReportData) (domain.Result, error) {
	p.calls++
	if !p.session.closed {
		panic("publishing before Git cleanup")
	}
	if p.err != nil {
		return domain.Result{}, p.err
	}
	p.output = data
	p.queue.analysis.Status = "completed"
	return domain.Result{}, nil
}
func setup(processor Processor, budget time.Duration) (*Service, *memoryQueue, *source, *publisher) {
	q := &memoryQueue{analysis: domain.Analysis{ID: domain.NewID(), Status: "queued"}}
	g := &source{s: &fakeSession{data: domain.GitData{Refs: []domain.GitRef{{Name: "refs/heads/main", ObjectSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}, Sources: []domain.SourceState{{Source: "git", Scope: "history", Status: "partial", Reason: &domain.Reason{Code: "GIT_COMMIT_LIMIT", Message: "limited", CannotConclude: "full history"}}}}}}
	p := &publisher{queue: q, session: g.s}
	return NewService(q, g, processor, p, budget), q, g, p
}
func usefulOutput() Output {
	value := domain.Scalar{}
	_ = value.UnmarshalJSON([]byte("false"))
	return Output{Snapshot: domain.SnapshotData{Inputs: []domain.MetricInput{{ID: "DOC_README", Status: "complete", RawValue: value}}, Sources: []domain.SourceState{{Source: "git", Scope: "history", Status: "complete"}}}}
}
func TestWorkerCompletesPartialWithoutRerunningOnRead(t *testing.T) {
	p := processorFunc(func(ctx context.Context, _ domain.Analysis, _ GitSession, advance func(string) error) (Output, error) {
		if err := advance("collecting_external"); err != nil {
			return Output{}, err
		}
		if err := advance("calculating"); err != nil {
			return Output{}, err
		}
		if err := advance("collecting_git"); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatal("stage regressed")
		}
		return usefulOutput(), nil
	})
	worker, q, git, publisher := setup(p, time.Second)
	for range 3 {
		if _, err := worker.RunOnce(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if q.analysis.Status != "completed" || q.claims != 1 || git.opens != 1 || publisher.calls != 1 || !git.s.closed {
		t.Fatal("job reran or failed")
	}
	if publisher.output.Sources[0].Status != "partial" {
		t.Fatal("processor upgraded partial Git data to complete")
	}
	if len(publisher.output.Refs) != 1 {
		t.Fatal("pinned refs not forwarded")
	}
	if len(q.stages) != 3 || q.stages[2] != "saving" {
		t.Fatal("server stages not recorded")
	}
}
func TestWorkerFailuresAndCleanup(t *testing.T) {
	for _, tc := range []struct {
		name              string
		processor         Processor
		saveErr, closeErr error
		code              string
		budget            time.Duration
	}{
		{name: "no processor", code: "ANALYSIS_FAILED", budget: time.Second},
		{name: "no usable input", processor: processorFunc(func(context.Context, domain.Analysis, GitSession, func(string) error) (Output, error) {
			return Output{}, nil
		}), code: "ANALYSIS_FAILED", budget: time.Second},
		{name: "panic", processor: processorFunc(func(context.Context, domain.Analysis, GitSession, func(string) error) (Output, error) {
			panic("raw source secret must not escape")
		}), code: "ANALYSIS_FAILED", budget: time.Second},
		{name: "save failure", processor: processorFunc(func(context.Context, domain.Analysis, GitSession, func(string) error) (Output, error) {
			return usefulOutput(), nil
		}), saveErr: errors.New("database details"), code: "REPORT_SAVE_FAILED", budget: time.Second},
		{name: "cleanup failure", processor: processorFunc(func(context.Context, domain.Analysis, GitSession, func(string) error) (Output, error) {
			return usefulOutput(), nil
		}), closeErr: errors.New("private path"), code: "ANALYSIS_FAILED", budget: time.Second},
		{name: "deadline", processor: processorFunc(func(ctx context.Context, _ domain.Analysis, _ GitSession, _ func(string) error) (Output, error) {
			<-ctx.Done()
			return Output{}, ctx.Err()
		}), code: "BUDGET_EXCEEDED", budget: 20 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			worker, q, git, publisher := setup(tc.processor, tc.budget)
			publisher.err = tc.saveErr
			git.s.closeErr = tc.closeErr
			worked, err := worker.RunOnce(t.Context())
			if err != nil || !worked {
				t.Fatalf("worked=%v err=%v", worked, err)
			}
			if q.analysis.Status != "failed" || q.analysis.Error == nil || q.analysis.Error.Code != tc.code || !git.s.closed {
				t.Fatalf("bad failure: %+v", q.analysis)
			}
			original := q.analysis.Error
			if _, err := worker.RunOnce(t.Context()); err != nil {
				t.Fatal(err)
			}
			if git.opens != 1 || q.analysis.Error != original {
				t.Fatal("terminal job changed")
			}
		})
	}
}

type fallbackProcessor struct{}

func (fallbackProcessor) Process(context.Context, domain.Analysis, GitSession, func(string) error) (Output, error) {
	output := usefulOutput()
	output.Report.Explanation.Mode = "deterministic_fallback"
	return output, nil
}
func (fallbackProcessor) Explain(ctx context.Context, _ domain.Analysis, _ Output) (Output, error) {
	<-ctx.Done()
	return Output{}, ctx.Err()
}

func TestExplanationTimeoutLeavesBudgetToPublishFallback(t *testing.T) {
	runner, queue, _, publisher := setup(fallbackProcessor{}, 200*time.Millisecond)
	if _, err := runner.RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if queue.analysis.Status != "completed" || publisher.calls != 1 {
		t.Fatal("explanation consumed the publication budget")
	}
	if len(queue.stages) != 2 || queue.stages[0] != "explaining" || queue.stages[1] != "saving" {
		t.Fatal("explanation stages not confirmed")
	}
}
