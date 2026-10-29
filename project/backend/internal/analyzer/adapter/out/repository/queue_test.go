package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/adapter/out/repository"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/storage"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func TestPostgresQueueClaimsAndTerminalStates(t *testing.T) {
	pool := testDatabase(t)
	snapshot, _ := loadFixtures(t)
	pg := repository.NewPostgres(pool)
	service := storage.NewService(pg)
	owner := domain.NewID()
	if _, err := pool.Exec(t.Context(), "INSERT INTO auth.principals(id,kind,created_at) VALUES ($1,'guest',now())", owner); err != nil {
		t.Fatal(err)
	}
	for range 12 {
		if _, err := service.CreateAccepted(t.Context(), identity(owner, "guest"), domain.NewID(), snapshot.Repository, snapshot.Versions); err != nil {
			t.Fatal(err)
		}
	}
	claimed := make(chan domain.Analysis, 12)
	failures := make(chan error, 12)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for {
				a, err := pg.Claim(t.Context(), time.Now(), time.Minute)
				if errors.Is(err, domain.ErrQueueEmpty) {
					return
				}
				if err != nil {
					failures <- err
					return
				}
				claimed <- a
			}
		})
	}
	wg.Wait()
	close(claimed)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var first domain.Analysis
	for a := range claimed {
		if seen[a.ID] {
			t.Fatal("a job was claimed twice")
		}
		seen[a.ID] = true
		first = a
		if a.Status != "running" || a.Stage == nil || *a.Stage != "collecting_git" || a.DeadlineAt == nil {
			t.Fatal("incomplete claim")
		}
	}
	if len(seen) != 12 {
		t.Fatalf("claims=%d", len(seen))
	}
	if err := pg.Advance(t.Context(), first.ID, "calculating", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := pg.Advance(t.Context(), first.ID, "collecting_git", time.Now()); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatal("stage regressed")
	}
	altered := first.Repository
	altered.GitHubID = "different"
	if err := pg.Pin(t.Context(), first.ID, altered); !errors.Is(err, domain.ErrInvalidData) {
		t.Fatal("accepted metadata changed")
	}
	if err := pg.Fail(t.Context(), first.ID, time.Now(), domain.Failure("ANALYSIS_FAILED", first.ID)); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		a, err := pg.Get(t.Context(), first.ID)
		if err != nil || a.Status != "failed" || a.Error == nil || a.Error.Code != "ANALYSIS_FAILED" || a.Stage != nil {
			t.Fatal("bad terminal record")
		}
	}
	if err := pg.Fail(t.Context(), first.ID, time.Now(), domain.Failure("REPORT_SAVE_FAILED", first.ID)); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatal("terminal failure overwritten")
	}
	if err := pg.Advance(t.Context(), first.ID, "saving", time.Now()); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatal("terminal stage changed")
	}
	if err := pg.Expire(t.Context(), time.Now().Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	for id := range seen {
		a, err := pg.Get(t.Context(), id)
		if err != nil || a.Status != "failed" || a.Error == nil {
			t.Fatal("crashed worker lease not terminal")
		}
		if id != first.ID && a.Error.Code != "BUDGET_EXCEEDED" {
			t.Fatal("expired job missing budget reason")
		}
	}
	if _, err := pg.Claim(t.Context(), time.Now(), time.Minute); !errors.Is(err, domain.ErrQueueEmpty) {
		t.Fatal("terminal job requeued")
	}
}

type collectedGit struct {
	metadata domain.RepositoryMetadata
	snapshot domain.SnapshotData
	closed   bool
}

func (g *collectedGit) Data() domain.GitData {
	sources := []domain.SourceState{}
	for _, s := range g.snapshot.Sources {
		if s.Source == "git" {
			sources = append(sources, s)
		}
	}
	return domain.GitData{Repository: g.metadata, Refs: g.snapshot.Refs, Sources: sources}
}
func (g *collectedGit) ReadBlob(context.Context, string) ([]byte, error) { return nil, nil }
func (g *collectedGit) Close() error                                     { g.closed = true; return nil }

type collectedSource struct {
	session *collectedGit
	calls   int
}

func (g *collectedSource) Open(_ context.Context, metadata domain.RepositoryMetadata) (worker.GitSession, error) {
	g.calls++
	g.session.metadata = metadata
	return g.session, nil
}

type fixtureProcessor struct {
	snapshot domain.SnapshotData
	report   domain.ReportData
}

func (p fixtureProcessor) Process(_ context.Context, _ domain.Analysis, _ worker.GitSession, advance func(string) error) (worker.Output, error) {
	if err := advance("collecting_external"); err != nil {
		return worker.Output{}, err
	}
	if err := advance("calculating"); err != nil {
		return worker.Output{}, err
	}
	return worker.Output{Snapshot: p.snapshot, Report: p.report}, nil
}

// Calculation fixtures test orchestration/publication only. Actual normalized
// inputs/calculators belong to #18479 and subsequent tasks; Git has its own real
// smart-HTTP control-repository tests.
func TestPostgresWorkerPublishesPartialResultOnce(t *testing.T) {
	pool := testDatabase(t)
	snapshot, report := loadFixtures(t)
	pg := repository.NewPostgres(pool)
	service := storage.NewService(pg)
	owner := domain.NewID()
	if _, err := pool.Exec(t.Context(), "INSERT INTO auth.principals(id,kind,created_at) VALUES ($1,'guest',now())", owner); err != nil {
		t.Fatal(err)
	}
	verified := identity(owner, "guest")
	analysis, err := service.CreateAccepted(t.Context(), verified, domain.NewID(), snapshot.Repository, snapshot.Versions)
	if err != nil {
		t.Fatal(err)
	}
	source := &collectedSource{session: &collectedGit{snapshot: snapshot.SnapshotData}}
	runner := worker.NewService(pg, source, fixtureProcessor{snapshot: snapshot.SnapshotData, report: report.ReportData}, service, time.Minute)
	worked, err := runner.RunOnce(t.Context())
	if err != nil || !worked {
		t.Fatalf("worker: %v", err)
	}
	completed, err := service.Get(t.Context(), verified, analysis.ID)
	if err != nil || completed.Status != "completed" || completed.ReportID == nil || !source.session.closed {
		t.Fatalf("not published: %+v %v", completed, err)
	}
	for range 3 {
		if _, err := service.GetResult(t.Context(), verified, analysis.ID); err != nil {
			t.Fatal(err)
		}
	}
	if worked, err := runner.RunOnce(t.Context()); err != nil || worked || source.calls != 1 {
		t.Fatal("reading/result replay executed job again")
	}
	var snapshots, reports int
	if err := pool.QueryRow(t.Context(), "SELECT (SELECT count(*) FROM analyzer.snapshots),(SELECT count(*) FROM report.reports)").Scan(&snapshots, &reports); err != nil {
		t.Fatal(err)
	}
	if snapshots != 1 || reports != 1 {
		t.Fatal("result published more than once")
	}
	if err := pg.Fail(t.Context(), analysis.ID, time.Now(), domain.Failure("REPORT_SAVE_FAILED", analysis.ID)); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatal("completed became failed")
	}
}
