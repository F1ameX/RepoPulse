package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/adapter/out/repository"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/snapshot"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/storage"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type snapshotSource struct {
	session *snapshotSession
	calls   int
}

func (s *snapshotSource) Open(_ context.Context, metadata domain.RepositoryMetadata) (worker.GitSession, error) {
	s.calls++
	s.session.data.Repository = metadata
	return s.session, nil
}

type snapshotSession struct {
	data   domain.GitData
	closed bool
}

func (s *snapshotSession) Data() domain.GitData { return s.data }
func (s *snapshotSession) ReadBlob(context.Context, string) ([]byte, error) {
	return nil, errors.New("no source text may be fetched by the snapshot assembler")
}
func (s *snapshotSession) Close() error { s.closed = true; return nil }

type snapshotCollector struct{ obtained time.Time }

func (c snapshotCollector) Collect(_ context.Context, a domain.Analysis, git worker.GitSession) (snapshot.Collection, error) {
	// A trusted control source, not production metric/scoring logic: derive a
	// confirmed false from the complete empty tree and persist numeric zero
	// alongside an external source whose acquisition time is genuinely unknown.
	var readme, zero domain.Scalar
	_ = json.Unmarshal([]byte("false"), &readme)
	_ = json.Unmarshal([]byte("0"), &zero)
	if len(git.Data().Files) != 0 {
		return snapshot.Collection{}, domain.ErrInvalidData
	}
	runAt := a.AssessmentAt.Add(-24 * time.Hour)
	actionsCount := 1
	return snapshot.Collection{
		GitHubCI: &domain.GitHubCI{
			Attempts: []domain.ActionsAttempt{
				{RunID: "9007199254740993", Attempt: 2, HeadSHA: *a.Repository.CommitSHA, Status: "completed", Conclusion: "success",
					CreatedAt: runAt, StartedAt: &runAt, UpdatedAt: runAt.Add(time.Minute), ObtainedAt: c.obtained},
			},
			Rules: []domain.BranchRule{}, ConfigPaths: []string{}, ConfigTreeComplete: true,
		},
		Sources: []domain.SourceState{
			{Source: "github", Scope: domain.ActionsScope, Status: "partial", ObtainedAt: &c.obtained,
				Sample: domain.Sample{Collected: &actionsCount},
				Reason: &domain.Reason{Code: "GITHUB_CI_INTERRUPTED", Message: "Следующая страница не получена", CannotConclude: "Полнота запусков неизвестна"}},
			{Source: "github", Scope: domain.BranchRulesScope, Status: "unavailable",
				Reason: &domain.Reason{Code: "insufficient_permissions", Message: "Источник недоступен", CannotConclude: "Настройки неизвестны"}},
		},
		Inputs: []snapshot.Input{
			{Value: domain.MetricInput{ID: "DOC_README", Unit: "bool", RawValue: readme, Status: "complete"}, Sources: []snapshot.Source{{Name: "git", Scope: "tree"}}},
			{Value: domain.MetricInput{ID: "DOC_SUPPORT_SHARE", Unit: "ratio", RawValue: zero, Status: "complete"}, Sources: []snapshot.Source{{Name: "git", Scope: "tree"}}},
		},
		Dependencies: []domain.Dependency{{Ecosystem: "Go", Package: "example.org/control", Version: "v1.2.3", ManifestPath: "go.mod", VersionSource: "go_require_declaration",
			Origins: []domain.DependencyOrigin{{ManifestPath: "go.mod", VersionSource: "go_require_declaration"}}}},
		Vulnerabilities: []domain.Vulnerability{{ID: "OSV-CONTROL", Ecosystem: "Go", Package: "example.org/control", Version: "v1.2.3", ObtainedAt: c.obtained}},
	}, nil
}

// Publication-only calculator: unknown scores are deliberate. No production
// formula is guessed here; real calculators are owned by neighboring tasks.
type snapshotCalculator struct{}

func (snapshotCalculator) Calculate(_ context.Context, a domain.Analysis, data domain.SnapshotData) (domain.ReportData, error) {
	reason := &domain.Reason{Code: "test_calculator", Message: "Контроль публикации", CannotConclude: "Баллы не вычислялись"}
	report := domain.ReportData{OverallReason: reason, Summary: "Контроль сохранения входов", Coverage: domain.Coverage{Total: 12, Unavailable: 12},
		Explanation: domain.Explanation{Mode: "deterministic_fallback", FallbackReason: text("no_findings"), CatalogVersion: a.Versions.RecommendationCatalog,
			SchemaVersion: a.Versions.ExplanationSchema, PromptVersion: a.Versions.Prompt}}
	for _, category := range []string{"activity", "documentation", "testing", "security", "ci_cd"} {
		item := domain.Category{ID: category, Status: "unavailable", Reason: reason}
		for _, input := range data.Inputs {
			matches := category == "activity" && strings.HasPrefix(input.ID, "ACT_") ||
				category == "documentation" && strings.HasPrefix(input.ID, "DOC_") ||
				category == "testing" && input.ID == "TEST_PRESENCE" ||
				category == "security" && strings.HasPrefix(input.ID, "SEC_") ||
				category == "ci_cd" && strings.HasPrefix(input.ID, "CI_")
			if matches {
				item.Metrics = append(item.Metrics, domain.Metric{Input: input, RuleVersion: a.Versions.Methodology})
			}
		}
		item.Coverage = domain.Coverage{Total: len(item.Metrics), Unavailable: len(item.Metrics)}
		report.Categories = append(report.Categories, item)
	}
	return report, nil
}

func text(value string) *string { return &value }

func TestNormalizedSnapshotIsPublishedAndReadOfflineForOriginalOwner(t *testing.T) {
	pool := testDatabase(t)
	fixture, _ := loadFixtures(t)
	pg := repository.NewPostgres(pool)
	service := storage.NewService(pg)
	owner := domain.NewID()
	if _, err := pool.Exec(t.Context(), "INSERT INTO auth.principals(id,kind,created_at) VALUES ($1,'guest',now())", owner); err != nil {
		t.Fatal(err)
	}
	verified := identity(owner, "guest")
	a, err := service.CreateAccepted(t.Context(), verified, domain.NewID(), fixture.Repository, fixture.Versions)
	if err != nil {
		t.Fatal(err)
	}
	obtained := domain.UTC(time.Now().Add(-time.Hour))
	zero := 0
	source := &snapshotSource{session: &snapshotSession{data: domain.GitData{
		Refs: []domain.GitRef{{Name: "refs/heads/main", ObjectSHA: *a.Repository.CommitSHA}},
		Sources: []domain.SourceState{{Source: "git", Scope: "tree", Status: "complete", ObtainedAt: &obtained,
			Sample: domain.Sample{Collected: &zero, Total: &zero}}},
	}}}
	processor := snapshot.NewProcessor([]snapshot.Collector{snapshotCollector{obtained}}, snapshotCalculator{})
	runner := worker.NewService(pg, source, processor, service, time.Minute)
	if worked, err := runner.RunOnce(t.Context()); err != nil || !worked {
		t.Fatalf("run: %v", err)
	}
	result, err := service.GetResult(t.Context(), verified, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	saved := result.Snapshot
	if saved.AnalysisID != a.ID || saved.Owner != a.Owner || saved.Versions != a.Versions || !saved.AssessmentAt.Equal(a.AssessmentAt) || !reflect.DeepEqual(saved.Window, a.Window) || !reflect.DeepEqual(saved.Repository, a.Repository) || !source.session.closed {
		t.Fatal("snapshot changed accepted metadata or was published before Git cleanup")
	}
	for index, want := range map[int]string{0: "null", 3: "false", 4: "0"} {
		raw, _ := json.Marshal(saved.Inputs[index].RawValue)
		if string(raw) != want {
			t.Fatalf("input %d = %s, want %s", index, raw, want)
		}
	}
	if saved.Inputs[1].ObservedAt != nil || saved.Vulnerabilities[0].Severity != nil || !saved.Vulnerabilities[0].ObtainedAt.Equal(obtained) {
		t.Fatal("unknown fields were fabricated or observation time changed")
	}
	if saved.GitHubCI == nil {
		t.Fatal("normalized CI inputs were not stored")
	}
	latest, found := saved.GitHubCI.LatestCompleted(saved.Window)
	if !found || latest.RunID != "9007199254740993" || latest.Attempt != 2 || latest.Conclusion != "success" || !latest.ObtainedAt.Equal(obtained) {
		t.Fatalf("Actions attempt changed on round trip: %+v", latest)
	}
	actions, rules := saved.Sources[1], saved.Sources[2]
	if actions.Scope != domain.ActionsScope || actions.Status != "partial" || actions.Reason == nil ||
		actions.Reason.Code != "GITHUB_CI_INTERRUPTED" || actions.Sample.Total != nil ||
		rules.Scope != domain.BranchRulesScope || rules.Status != "unavailable" || rules.ObtainedAt != nil ||
		rules.Sample.Collected != nil || saved.GitHubCI.RequiredChecks(rules) != nil {
		t.Fatalf("CI source completeness changed on round trip: actions=%+v rules=%+v", actions, rules)
	}
	if !result.Report.CompletedAt.After(a.CreatedAt) || result.Report.AnalysisID != a.ID || result.Report.SnapshotID != saved.ID {
		t.Fatal("completion metadata not generated from this execution")
	}
	encoded, _ := json.Marshal(result)
	// The source changes and disappears; reading cannot consult it again.
	source.session.data.Refs[0].ObjectSHA = strings.Repeat("f", 40)
	*source.session.data.Sources[0].Sample.Collected = 999
	for range 3 {
		again, err := service.GetResult(t.Context(), verified, a.ID)
		if err != nil {
			t.Fatal(err)
		}
		actual, _ := json.Marshal(again)
		if string(actual) != string(encoded) || !reflect.DeepEqual(again.Snapshot.GitHubCI, saved.GitHubCI) || source.calls != 1 {
			t.Fatal("offline read changed snapshot or re-fetched the source")
		}
	}
	if _, err := service.GetResult(t.Context(), identity(domain.NewID(), "guest"), a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("snapshot readable by another owner")
	}
	var snapshots, reports int
	if err := pool.QueryRow(t.Context(), "SELECT (SELECT count(*) FROM analyzer.snapshots),(SELECT count(*) FROM report.reports)").Scan(&snapshots, &reports); err != nil || snapshots != 1 || reports != 1 {
		t.Fatal("snapshot and report were not published once together")
	}
}
