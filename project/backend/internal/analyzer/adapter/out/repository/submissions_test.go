package repository_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/adapter/out/repository"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/analysis"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type submissionSource func(context.Context, domain.RepositoryURL) (domain.RepositoryMetadata, error)

func (f submissionSource) LookupForAnalysis(ctx context.Context, address domain.RepositoryURL) (domain.RepositoryMetadata, error) {
	return f(ctx, address)
}

func TestSubmissionReservationPreventsDuplicateGitHubAndJobs(t *testing.T) {
	pool := testDatabase(t)
	snapshot, _ := loadFixtures(t)
	ownerID := domain.NewID()
	if _, err := pool.Exec(t.Context(), "INSERT INTO auth.principals(id,kind,created_at) VALUES($1,'guest',$2)", ownerID, time.Now()); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour)
	verified := domain.VerifiedContext{GuestSessionID: &ownerID, GuestExpiresAt: &expires}
	var calls atomic.Int32
	service := analysis.NewService(repository.NewPostgres(pool), submissionSource(func(context.Context, domain.RepositoryURL) (domain.RepositoryMetadata, error) {
		calls.Add(1)
		return snapshot.Repository, nil
	}), snapshot.Versions, 5*time.Second)
	command := analysis.CreateAnalysisCommand{Context: verified, Request: analysis.CreateAnalysisRequest{RequestID: domain.NewID(), RepositoryURL: snapshot.Repository.CanonicalURL}}
	var wait sync.WaitGroup
	results := make(chan domain.Analysis, 8)
	errorsFound := make(chan error, 8)
	for range 8 {
		wait.Go(func() {
			a, err := service.CreateAnalysis(t.Context(), command)
			if err != nil {
				errorsFound <- err
			} else {
				results <- a
			}
		})
	}
	wait.Wait()
	close(results)
	close(errorsFound)
	for err := range errorsFound {
		if !errors.Is(err, domain.ErrSubmissionPending) {
			t.Fatalf("concurrent submission: %#v", err)
		}
	}
	firstID := ""
	for a := range results {
		if firstID != "" && a.ID != firstID {
			t.Fatal("multiple analysis IDs for one submission")
		}
		firstID = a.ID
	}
	again, err := service.CreateAnalysis(t.Context(), command)
	if err != nil || firstID == "" || again.ID != firstID || calls.Load() != 1 {
		t.Fatalf("replay=%+v error=%#v calls=%d", again, err, calls.Load())
	}
	var jobs int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM analyzer.analyses WHERE owner_id=$1", ownerID).Scan(&jobs); err != nil || jobs != 1 {
		t.Fatalf("jobs=%d error=%#v", jobs, err)
	}
	command.Request.RepositoryURL = "https://github.com/other/repository"
	if _, err := service.CreateAnalysis(t.Context(), command); !errors.Is(err, domain.ErrSubmissionChanged) || calls.Load() != 1 {
		t.Fatalf("changed URL error=%#v calls=%d", err, calls.Load())
	}
	// Persisted terminal failures and stages are read, never reconstructed from I/O.
	_, err = pool.Exec(t.Context(), `UPDATE analyzer.analyses SET status='failed',stage='saving',completed_at=now(),
		error='{"code":"REPORT_SAVE_FAILED","message":"private detail","trace_id":"worker-trace"}'::jsonb WHERE id=$1`, firstID)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := service.GetAnalysis(t.Context(), analysis.GetAnalysisQuery{Context: verified, AnalysisID: firstID})
	if err != nil || stored.Failure == nil || stored.Failure.Code != "REPORT_SAVE_FAILED" || stored.Stage == nil || *stored.Stage != "saving" {
		t.Fatalf("persisted terminal status=%+v error=%#v", stored, err)
	}
}
