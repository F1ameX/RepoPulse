package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/adapter/out/repository"
	analyzerapp "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/app"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/storage"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	authdomain "github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
)

const backendRoot = "../../../../../"

func loadFixtures(t *testing.T) (domain.Snapshot, domain.Report) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(backendRoot, "../../contracts/examples/snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := domain.DecodeSnapshot(data)
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(backendRoot, "../../contracts/examples/report-partial.json"))
	if err != nil {
		t.Fatal(err)
	}
	var report domain.Report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	return snapshot, report
}

func migrate(t *testing.T, pool *pgxpool.Pool, down bool) {
	t.Helper()
	names := []string{"auth", "analyzer", "report"}
	if down {
		names = []string{"report", "analyzer", "auth"}
	}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(backendRoot, "migrations", name, "00001_foundation.sql"))
		if err != nil {
			t.Fatal(err)
		}
		up, rollback, ok := strings.Cut(string(data), "-- +goose Down")
		if !ok {
			t.Fatal("missing Down")
		}
		sql := strings.TrimPrefix(up, "-- +goose Up")
		if down {
			sql = rollback
		}
		if _, err := pool.Exec(t.Context(), sql); err != nil {
			t.Fatalf("migration %s down=%v: %v", name, down, err)
		}
	}
}

func testDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("REPOPULSE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set REPOPULSE_TEST_DATABASE_URL to run isolated PostgreSQL integration checks")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	admin, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	name := "repopulse_storage_" + strings.ReplaceAll(domain.NewID(), "-", "")
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(t.Context(), "CREATE DATABASE "+quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	var pool *pgxpool.Pool
	t.Cleanup(func() {
		if pool != nil {
			pool.Close()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.Exec(ctx, "DROP DATABASE "+quoted+" WITH (FORCE)"); err != nil {
			t.Errorf("drop isolated test DB: %v", err)
		}
		admin.Close()
	})
	testDSN := dsn + " dbname=" + name
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, parseErr := url.Parse(dsn)
		if parseErr != nil {
			t.Fatal("invalid test database URL")
		}
		parsed.Path = "/" + name
		parsed.RawPath = ""
		query := parsed.Query()
		query.Del("dbname")
		parsed.RawQuery = query.Encode()
		testDSN = parsed.String()
	}
	cfg, err = pgxpool.ParseConfig(testDSN)
	if err != nil {
		t.Fatal("invalid isolated database configuration")
	}
	// Do not run any schema operation unless the DSN selects exactly the
	// newly created disposable database, even if query parameters override it.
	if cfg.ConnConfig.Database != name {
		t.Fatal("test DSN does not select the isolated database")
	}
	pool, err = pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	migrate(t, pool, false)
	return pool
}

func identity(id, kind string) authdomain.VerifiedContext {
	expires := time.Now().Add(time.Hour)
	if kind == "account" {
		return authdomain.VerifiedContext{UserID: &id, AccountExpiresAt: &expires}
	}
	return authdomain.VerifiedContext{GuestSessionID: &id, GuestExpiresAt: &expires}
}

func TestPostgresStorage(t *testing.T) {
	pool := testDatabase(t)
	ctx := t.Context()
	snapshot, report := loadFixtures(t)
	pg := repository.NewPostgres(pool)
	service := storage.NewService(pg)
	newAnalysis := func(t *testing.T, kind string) (domain.Analysis, authdomain.VerifiedContext) {
		t.Helper()
		ownerID := domain.NewID()
		if _, err := pool.Exec(ctx, "INSERT INTO auth.principals (id,kind,created_at) VALUES ($1,$2,$3)", ownerID, kind, time.Now()); err != nil {
			t.Fatal(err)
		}
		verified := identity(ownerID, kind)
		a, err := service.CreateAccepted(ctx, verified, domain.NewID(), snapshot.Repository, snapshot.Versions)
		if err != nil {
			t.Fatal(err)
		}
		return a, verified
	}
	complete := func(t *testing.T, a domain.Analysis) domain.Result {
		t.Helper()
		if err := service.Start(ctx, a.ID, time.Minute); err != nil {
			t.Fatal(err)
		}
		result, err := service.Complete(ctx, a.ID, snapshot.SnapshotData, report.ReportData)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	t.Run("round trip and frozen owner IDs timestamps versions", func(t *testing.T) {
		a, verified := newAnalysis(t, "guest")
		result := complete(t, a)
		read, err := service.GetResult(ctx, verified, a.ID)
		if err != nil {
			t.Fatal(err)
		}
		left, _ := json.Marshal(result)
		right, _ := json.Marshal(read)
		if string(left) != string(right) {
			t.Fatalf("round trip mismatch\n%s\n%s", left, right)
		}
		got, err := service.Get(ctx, verified, a.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Owner != a.Owner || got.ID != a.ID || got.SubmissionID != a.SubmissionID ||
			!got.AssessmentAt.Equal(a.AssessmentAt) || got.Versions != a.Versions || got.ReportID == nil || *got.ReportID != result.Report.ID || got.CompletedAt == nil {
			t.Fatalf("metadata changed: %+v", got)
		}
		var dependencies int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM analyzer.snapshot_dependencies WHERE snapshot_id=$1", read.Snapshot.ID).Scan(&dependencies); err != nil || dependencies != 3 {
			t.Fatalf("dependency rows=%d, err=%v", dependencies, err)
		}
		// Account login does not transfer a guest result; the original guest may
		// still read it while its own context is valid.
		combined := identity(domain.NewID(), "account")
		combined.GuestSessionID, combined.GuestExpiresAt = verified.GuestSessionID, verified.GuestExpiresAt
		if _, err := service.GetResult(ctx, combined, a.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := service.GetResult(ctx, identity(domain.NewID(), "account"), a.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("foreign owner read: %v", err)
		}
		if _, err := service.Complete(ctx, a.ID, snapshot.SnapshotData, report.ReportData); !errors.Is(err, domain.ErrAlreadyCompleted) {
			t.Fatalf("overwrite: %v", err)
		}
		if err := service.Start(ctx, a.ID, time.Minute); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatalf("restart: %v", err)
		}
	})
	t.Run("same request remains the same analysis", func(t *testing.T) {
		a, verified := newAnalysis(t, "account")
		again, err := service.CreateAccepted(ctx, verified, a.RequestID, snapshot.Repository, snapshot.Versions)
		if err != nil || again.ID != a.ID || !again.AssessmentAt.Equal(a.AssessmentAt) {
			t.Fatalf("replay: %+v %v", again, err)
		}
		different := snapshot.Repository
		different.Name = "Other"
		different.CanonicalURL = "https://github.com/example/other"
		if _, err := service.CreateAccepted(ctx, verified, a.RequestID, different, snapshot.Versions); !errors.Is(err, domain.ErrSubmissionChanged) {
			t.Fatalf("changed URL: %v", err)
		}
	})
	t.Run("account priority expiry and unknown revision", func(t *testing.T) {
		guest, guestContext := newAnalysis(t, "guest")
		account, accountContext := newAnalysis(t, "account")
		both := accountContext
		both.GuestSessionID, both.GuestExpiresAt = guestContext.GuestSessionID, guestContext.GuestExpiresAt
		unknown := snapshot.Repository
		unknown.DefaultBranch, unknown.CommitSHA = nil, nil
		accepted, err := service.CreateAccepted(ctx, both, domain.NewID(), unknown, snapshot.Versions)
		if err != nil || accepted.Owner != account.Owner {
			t.Fatalf("account priority: %+v %v", accepted, err)
		}
		result := complete(t, accepted)
		if result.Snapshot.Repository.DefaultBranch != nil || result.Report.Repository.CommitSHA != nil {
			t.Fatal("unknown revision replaced")
		}
		expired := time.Now().Add(-time.Second)
		both.AccountExpiresAt = &expired
		if _, err := service.CreateAccepted(ctx, both, domain.NewID(), unknown, snapshot.Versions); !errors.Is(err, domain.ErrExpiredOwner) {
			t.Fatalf("silent guest fallback: %v", err)
		}
		if _, err := service.CreateAccepted(ctx, authdomain.VerifiedContext{}, domain.NewID(), unknown, snapshot.Versions); !errors.Is(err, domain.ErrUnverifiedOwner) {
			t.Fatalf("missing identity: %v", err)
		}
		if got, err := service.Get(ctx, guestContext, guest.ID); err != nil || got.Owner != guest.Owner {
			t.Fatalf("guest ownership changed: %+v %v", got, err)
		}
	})
	t.Run("concurrent submission replay preserves first metadata", func(t *testing.T) {
		a, verified := newAnalysis(t, "account")
		requestID := domain.NewID()
		var wg sync.WaitGroup
		results := make(chan domain.Analysis, 8)
		failures := make(chan error, 8)
		for range 8 {
			wg.Go(func() {
				got, err := service.CreateAccepted(ctx, verified, requestID, snapshot.Repository, snapshot.Versions)
				if err != nil {
					failures <- err
					return
				}
				results <- got
			})
		}
		wg.Wait()
		close(results)
		close(failures)
		for err := range failures {
			t.Fatal(err)
		}
		var first domain.Analysis
		for got := range results {
			if first.ID == "" {
				first = got
			}
			if got.ID != first.ID || got.SubmissionID != first.SubmissionID || !got.AssessmentAt.Equal(first.AssessmentAt) {
				t.Fatal("replay changed metadata")
			}
		}
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM analyzer.submissions WHERE owner_id=$1 AND request_id=$2", a.Owner.ID, requestID).Scan(&count); err != nil || count != 1 {
			t.Fatalf("duplicate submission: %d %v", count, err)
		}
	})
	t.Run("vulnerability with unknown severity round trip", func(t *testing.T) {
		a, verified := newAnalysis(t, "account")
		if err := service.Start(ctx, a.ID, time.Minute); err != nil {
			t.Fatal(err)
		}
		data := snapshot.SnapshotData
		d := data.Dependencies[0]
		data.Vulnerabilities = []domain.Vulnerability{{ID: "OSV-fixture", Ecosystem: d.Ecosystem, Package: d.Package,
			Version: d.Version, ObtainedAt: time.Now(), Severity: nil, EvidenceURL: nil}}
		if _, err := service.Complete(ctx, a.ID, data, report.ReportData); err != nil {
			t.Fatal(err)
		}
		got, err := service.GetResult(ctx, verified, a.ID)
		if err != nil || len(got.Snapshot.Vulnerabilities) != 1 || got.Snapshot.Vulnerabilities[0].Severity != nil {
			t.Fatalf("vulnerability: %+v %v", got, err)
		}
		var severity *string
		if err := pool.QueryRow(ctx, "SELECT severity FROM analyzer.snapshot_vulnerabilities WHERE snapshot_id=$1", got.Snapshot.ID).Scan(&severity); err != nil || severity != nil {
			t.Fatalf("relational severity: %v %v", severity, err)
		}
	})
	t.Run("SQL JSON keeps false zero and unknown distinct", func(t *testing.T) {
		for _, unknownScore := range []bool{false, true} {
			a, verified := newAnalysis(t, "account")
			if err := service.Start(ctx, a.ID, time.Minute); err != nil {
				t.Fatal(err)
			}
			facts, output := loadFixtures(t)
			if err := json.Unmarshal([]byte("false"), &facts.Inputs[3].RawValue); err != nil {
				t.Fatal(err)
			}
			for i := range output.Categories {
				for j := range output.Categories[i].Metrics {
					if output.Categories[i].Metrics[j].Input.ID == "DOC_README" {
						output.Categories[i].Metrics[j].Input = facts.Inputs[3]
					}
				}
			}
			zero := 0.0
			output.OverallScore = &zero
			if unknownScore {
				output.OverallScore = nil
				output.OverallReason = &domain.Reason{Code: "unknown", Message: "No score", CannotConclude: "No overall assessment"}
			}
			if _, err := service.Complete(ctx, a.ID, facts.SnapshotData, output.ReportData); err != nil {
				t.Fatal(err)
			}
			got, err := service.GetResult(ctx, verified, a.ID)
			if err != nil {
				t.Fatal(err)
			}
			for i, want := range map[int]string{0: "null", 3: "false", 4: "0"} {
				raw, _ := json.Marshal(got.Snapshot.Inputs[i].RawValue)
				if string(raw) != want {
					t.Fatalf("metric %d got %s want %s", i, raw, want)
				}
			}
			var sqlScore *float64
			if err := pool.QueryRow(ctx, "SELECT overall_score::float8 FROM report.reports WHERE analysis_id=$1", a.ID).Scan(&sqlScore); err != nil {
				t.Fatal(err)
			}
			if unknownScore {
				if sqlScore != nil || got.Report.OverallScore != nil {
					t.Fatal("unknown score replaced")
				}
			} else if sqlScore == nil || *sqlScore != 0 {
				t.Fatal("confirmed zero lost")
			}
		}
	})
	t.Run("concurrent completion has exactly one winner", func(t *testing.T) {
		a, verified := newAnalysis(t, "account")
		if err := service.Start(ctx, a.ID, time.Minute); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		results := make(chan error, 8)
		for range 8 {
			wg.Go(func() {
				_, err := service.Complete(ctx, a.ID, snapshot.SnapshotData, report.ReportData)
				results <- err
			})
		}
		wg.Wait()
		close(results)
		winners := 0
		for err := range results {
			if err == nil {
				winners++
			} else if !errors.Is(err, domain.ErrAlreadyCompleted) {
				t.Fatal(err)
			}
		}
		if winners != 1 {
			t.Fatalf("winners=%d", winners)
		}
		if _, err := service.GetResult(ctx, verified, a.ID); err != nil {
			t.Fatal(err)
		}
		for _, table := range []string{"analyzer.snapshots", "report.reports"} {
			var count int
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE analysis_id=$1", a.ID).Scan(&count); err != nil || count != 1 {
				t.Fatalf("%s count=%d err=%v", table, count, err)
			}
		}
	})
	t.Run("failed final insert rolls back snapshot and dependencies", func(t *testing.T) {
		a, verified := newAnalysis(t, "guest")
		if err := service.Start(ctx, a.ID, time.Minute); err != nil {
			t.Fatal(err)
		}
		// A temporary test constraint forces failure after snapshot/dependency inserts.
		if _, err := pool.Exec(ctx, "ALTER TABLE report.reports ADD CONSTRAINT reject_test_result CHECK (false) NOT VALID"); err != nil {
			t.Fatal(err)
		}
		_, writeErr := service.Complete(ctx, a.ID, snapshot.SnapshotData, report.ReportData)
		if _, err := pool.Exec(ctx, "ALTER TABLE report.reports DROP CONSTRAINT reject_test_result"); err != nil {
			t.Fatal(err)
		}
		if writeErr == nil {
			t.Fatal("injected failure was ignored")
		}
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM analyzer.snapshots WHERE analysis_id=$1", a.ID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("partial snapshot: %d %v", count, err)
		}
		got, err := service.Get(ctx, verified, a.ID)
		if err != nil || got.Status != "running" || got.ReportID != nil || got.CompletedAt != nil {
			t.Fatalf("partial completion: %+v %v", got, err)
		}
		if _, err := service.Complete(ctx, a.ID, snapshot.SnapshotData, report.ReportData); err != nil {
			t.Fatalf("retry after rollback: %v", err)
		}
	})
	t.Run("locked metadata rejects direct repository forgery", func(t *testing.T) {
		a, _ := newAnalysis(t, "account")
		if err := service.Start(ctx, a.ID, time.Minute); err != nil {
			t.Fatal(err)
		}
		a, err := pg.Get(ctx, a.ID)
		if err != nil {
			t.Fatal(err)
		}
		result, err := domain.NewResult(a, domain.NewID(), domain.NewID(), time.Now(), snapshot.SnapshotData, report.ReportData)
		if err != nil {
			t.Fatal(err)
		}
		result.Snapshot.Owner.ID = domain.NewID()
		if err := pg.Complete(ctx, result); !errors.Is(err, domain.ErrInvalidData) {
			t.Fatalf("owner forgery: %v", err)
		}
	})
	t.Run("application wiring opens existing storage", func(t *testing.T) {
		a, verified := newAnalysis(t, "account")
		wired, closeStorage, err := analyzerapp.OpenStorage(ctx, analyzerapp.Config{
			DatabaseURL: pool.Config().ConnString(), DatabaseTimeout: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		defer closeStorage()
		got, err := wired.Get(ctx, verified, a.ID)
		if err != nil || got.ID != a.ID {
			t.Fatalf("wired read: %+v %v", got, err)
		}
	})
	// Verify the same checked-in migration set supports rollback and a clean replay.
	migrate(t, pool, true)
	if _, _, err := analyzerapp.OpenStorage(ctx, analyzerapp.Config{
		DatabaseURL: pool.Config().ConnString(), DatabaseTimeout: time.Second}); err == nil {
		t.Fatal("application silently accepted missing migrations")
	}
	migrate(t, pool, false)
}
