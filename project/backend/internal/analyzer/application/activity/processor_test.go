package activity

import (
	"context"
	"math"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/scoring"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type controlSession struct{ data domain.GitData }

func (s controlSession) Data() domain.GitData { return s.data }
func (s controlSession) ReadBlob(context.Context, string) ([]byte, error) {
	return nil, domain.ErrInvalidData
}
func (s controlSession) Close() error { return nil }

func runningAnalysis(t *testing.T) domain.Analysis {
	t.Helper()
	body, err := os.ReadFile("../../../../../../contracts/examples/snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := domain.DecodeSnapshot(body)
	if err != nil {
		t.Fatal(err)
	}
	return domain.Analysis{ID: fixture.AnalysisID, SubmissionID: domain.NewID(), RequestID: domain.NewID(), Owner: fixture.Owner, Repository: fixture.Repository,
		AssessmentAt: fixture.AssessmentAt, CreatedAt: fixture.AssessmentAt, Window: fixture.Window, Versions: fixture.Versions, Status: "running", StartedAt: &fixture.AssessmentAt}
}

func TestPartialHistoryPreservesIndependentLastCommitProof(t *testing.T) {
	a := runningAnalysis(t)
	engine, err := scoring.Default()
	if err != nil {
		t.Fatal(err)
	}
	data := domain.GitData{Repository: a.Repository, MainCommits: []domain.ActivityCommit{{SHA: *a.Repository.CommitSHA, CommittedAt: a.AssessmentAt.Add(-9 * 24 * time.Hour)}},
		Sources: []domain.SourceState{{Source: "git", Scope: domain.ActivityHistoryScope, Status: "partial", Reason: &domain.Reason{
			Code: "GIT_FETCH_FAILED", Message: "Другие refs не получены", CannotConclude: "Полнота окна неизвестна"}}}}
	for _, confirmed := range []bool{false, true} {
		data.MainWalkComplete = confirmed
		output, err := NewProcessor(engine).Process(t.Context(), a, controlSession{data}, func(string) error { return nil })
		if err != nil {
			t.Fatal(err)
		}
		last := output.Report.Categories[0].Metrics[1]
		if confirmed {
			if last.Input.Status != "complete" || last.NormalizedScore == nil || last.Input.ObservedAt == nil {
				t.Fatal("independent proof was lost during snapshot assembly")
			}
		} else {
			raw, _ := last.Input.RawValue.MarshalJSON()
			if string(raw) != "null" || last.NormalizedScore != nil || last.Input.ObservedAt != nil {
				t.Fatal("one observed commit falsely proved the last date")
			}
		}
	}
}

type manifestSession struct{ controlSession }

func (s manifestSession) ReadBlob(context.Context, string) ([]byte, error) {
	return []byte("module example.com/root\nrequire example.com/lib v1.2.3\n"), nil
}

// This is the processor constructed by app.Run, not the optional RunWithInputs path.
func TestRuntimeProcessorPreservesDependenciesInSnapshot(t *testing.T) {
	body, err := os.ReadFile("../../../../../../contracts/examples/snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := domain.DecodeSnapshot(body)
	if err != nil {
		t.Fatal(err)
	}
	a := domain.Analysis{ID: fixture.AnalysisID, SubmissionID: domain.NewID(), RequestID: domain.NewID(), Owner: fixture.Owner, Repository: fixture.Repository,
		AssessmentAt: fixture.AssessmentAt, CreatedAt: fixture.AssessmentAt, Window: fixture.Window, Versions: fixture.Versions, Status: "running"}
	engine, err := scoring.Default()
	if err != nil {
		t.Fatal(err)
	}
	git := domain.GitData{Repository: a.Repository, Files: []domain.GitFile{{Path: "go.mod", ObjectSHA: "manifest", Mode: "100644"}},
		MainWalkComplete: true, MainCommits: []domain.ActivityCommit{{SHA: *a.Repository.CommitSHA, CommittedAt: a.AssessmentAt.Add(-time.Hour)}},
		Sources: []domain.SourceState{{Source: "git", Scope: "tree", Status: "complete"}, {Source: "git", Scope: domain.ActivityHistoryScope, Status: "complete"}}}
	output, err := NewProcessor(engine).Process(t.Context(), a, manifestSession{controlSession{git}}, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(output.Snapshot.Dependencies) != 1 {
		t.Fatalf("%+v", output.Snapshot)
	}
	d := output.Snapshot.Dependencies[0]
	if d.Ecosystem != "Go" || d.Package != "example.com/lib" || d.Version != "v1.2.3" || d.ManifestPath != "go.mod" || d.VersionSource != "go_require_declaration" || len(d.Origins) != 1 {
		t.Fatalf("%+v", d)
	}
	found := false
	for _, s := range output.Snapshot.Sources {
		if s.Scope == "dependencies" && s.Status == "complete" {
			found = true
		}
	}
	if !found {
		t.Fatal("dependency collection state lost during snapshot.Build")
	}
}

type fixedOSV func(context.Context, []domain.OSVQuery) []domain.OSVResult

func (f fixedOSV) Query(ctx context.Context, queries []domain.OSVQuery) []domain.OSVResult {
	return f(ctx, queries)
}

type historySession struct{ manifestSession }

func (historySession) HistoryTree(context.Context, string) ([]domain.GitFile, error) {
	return nil, nil
}

func TestRuntimeKeepsSecretScanWhenOSVUnavailable(t *testing.T) {
	a := runningAnalysis(t)
	engine, err := scoring.Default()
	if err != nil {
		t.Fatal(err)
	}
	git := domain.GitData{Repository: a.Repository,
		Files:   []domain.GitFile{{Path: "go.mod", ObjectSHA: "manifest", Mode: "100644"}},
		Commits: []domain.GitCommit{{SHA: *a.Repository.CommitSHA}},
		Sources: []domain.SourceState{
			{Source: "git", Scope: domain.GitTreeScope, Status: "complete"},
			{Source: "git", Scope: "refs", Status: "complete"},
			{Source: "git", Scope: "history", Status: "complete"},
		},
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	coreDeadline, _ := ctx.Deadline()
	called := false
	client := fixedOSV(func(queryCtx context.Context, queries []domain.OSVQuery) []domain.OSVResult {
		called = true
		deadline, ok := queryCtx.Deadline()
		if !ok || coreDeadline.Sub(deadline) < 10*time.Second || len(queries) != 1 {
			t.Fatalf("OSV did not leave room for the history scan: deadline %v, queries %v", deadline, queries)
		}
		return []domain.OSVResult{{Query: queries[0], ReasonCode: "OSV_ATTEMPT_LIMIT"}}
	})
	session := historySession{manifestSession{controlSession{git}}}
	out, err := NewProcessorWithSources(engine, nil, client).Process(ctx, a, session, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	security := out.Report.Categories[3]
	scan, osv := security.Metrics[0], security.Metrics[1]
	if !called || scan.Input.ID != "SEC_SECRETS" || scan.Input.Status != "complete" || scan.NormalizedScore == nil ||
		osv.Input.ID != "SEC_VULN_RISK" || osv.Input.Status != "unavailable" || osv.Input.Reason == nil ||
		osv.Input.Reason.Code != "OSV_ATTEMPT_LIMIT" || osv.NormalizedScore != nil {
		t.Fatalf("unexpected security metrics: %+v", security.Metrics)
	}
	if _, err := domain.NewResult(a, domain.NewID(), domain.NewID(), a.AssessmentAt, out.Snapshot, out.Report); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeVulnerabilityRisk(t *testing.T) {
	a := runningAnalysis(t)
	engine, err := scoring.Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, raw, status string
		ids               []string
		levels            map[string]string
		reason            string
		score             float64
		findings          int
	}{
		{"duplicate", "15", "complete", []string{"A", "A", "B"}, map[string]string{"A": "critical", "B": "high"}, "", 36.787944117144235, 2},
		{"clear", "0", "complete", nil, nil, "", 100, 0},
		{"unknown", "null", "partial", []string{"A"}, nil, "", 0, 1},
		{"interrupted", "null", "partial", []string{"A"}, map[string]string{"A": "critical"}, "OSV_ATTEMPT_LIMIT", 0, 1},
		{"empty", "null", "not_applicable", nil, nil, "", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			git := domain.GitData{Repository: a.Repository, Files: []domain.GitFile{{Path: "go.mod", ObjectSHA: "manifest", Mode: "100644"}},
				Sources: []domain.SourceState{{Source: "git", Scope: domain.GitTreeScope, Status: "complete"}}}
			if tc.name == "empty" {
				git.Files = nil
			}
			client := fixedOSV(func(_ context.Context, queries []domain.OSVQuery) []domain.OSVResult {
				if len(queries) != 1 || queries[0].Ecosystem != "Go" || tc.name == "empty" {
					t.Fatalf("unexpected queries: %+v", queries)
				}
				return []domain.OSVResult{{Query: queries[0], ReasonCode: tc.reason, Pages: []domain.OSVPage{
					{VulnerabilityIDs: tc.ids, Severity: tc.levels, ObtainedAt: a.AssessmentAt},
				}}}
			})
			out, err := NewProcessorWithSources(engine, nil, client).Process(t.Context(), a, manifestSession{controlSession{git}}, func(string) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			published, err := domain.NewResult(a, domain.NewID(), domain.NewID(), a.AssessmentAt, out.Snapshot, out.Report)
			if err != nil {
				t.Fatal(err)
			}
			replayed, err := engine.Replay(t.Context(), published.Snapshot)
			if err != nil {
				t.Fatal(err)
			}
			metric := out.Report.Categories[3].Metrics[1]
			if !reflect.DeepEqual(published.Report.Categories[3].Metrics[1], replayed.Categories[3].Metrics[1]) ||
				!reflect.DeepEqual(published.Report.Findings, replayed.Findings) {
				t.Fatal("replay changed the OSV metric or findings")
			}
			raw, _ := metric.Input.RawValue.MarshalJSON()
			if metric.Input.ID != "SEC_VULN_RISK" || string(raw) != tc.raw || metric.Input.Status != tc.status {
				t.Fatalf("unexpected metric: %+v, raw %s", metric, raw)
			}
			if tc.status == "complete" {
				if metric.NormalizedScore == nil || math.Abs(*metric.NormalizedScore-tc.score) > 1e-9 {
					t.Fatalf("unexpected score: %v", metric.NormalizedScore)
				}
			} else if metric.NormalizedScore != nil || metric.Input.Reason == nil {
				t.Fatal("N/A must have a reason and no score")
			}
			if len(metric.Input.Evidence) != tc.findings || len(out.Snapshot.Vulnerabilities) != tc.findings {
				t.Fatal("findings were lost or duplicated")
			}
			if tc.findings > 0 && (len(out.Report.Findings) != 1 || metric.Input.Evidence[0].Dependency.VersionSource != "go_require_declaration") {
				t.Fatal("report lost vulnerability provenance")
			}
		})
	}
}

type mixedSession struct{ controlSession }

func (s mixedSession) ReadBlob(_ context.Context, object string) ([]byte, error) {
	switch object {
	case "npm":
		return []byte(`{"dependencies":{"example-package":"^1.0.0"}}`), nil
	case "lock":
		return []byte(`{"lockfileVersion":3,"packages":{"":{"dependencies":{"example-package":"^1.0.0"}},"node_modules/example-package":{"version":"1.2.3"}}}`), nil
	case "pom":
		return []byte(`<project><dependencies><dependency><groupId>example</groupId><artifactId>lib</artifactId><version>1.2.3</version></dependency></dependencies></project>`), nil
	case "go":
		return []byte("module example.com/root\nrequire example.com/lib v1.2.3\n"), nil
	default:
		return nil, domain.ErrInvalidData
	}
}

func TestRuntimeVulnerabilityRiskMixedProfiles(t *testing.T) {
	a := runningAnalysis(t)
	engine, err := scoring.Default()
	if err != nil {
		t.Fatal(err)
	}
	git := domain.GitData{Repository: a.Repository,
		Sources: []domain.SourceState{{Source: "git", Scope: domain.GitTreeScope, Status: "complete"}},
		Files: []domain.GitFile{
			{Path: "package.json", ObjectSHA: "npm", Mode: "100644"},
			{Path: "package-lock.json", ObjectSHA: "lock", Mode: "100644"},
			{Path: "pom.xml", ObjectSHA: "pom", Mode: "100644"},
			{Path: "go.mod", ObjectSHA: "go", Mode: "100644"},
		},
	}
	want := map[string]domain.OSVQuery{
		"Go":    {Ecosystem: "Go", Name: "example.com/lib", Version: "v1.2.3"},
		"Maven": {Ecosystem: "Maven", Name: "example:lib", Version: "1.2.3"},
		"npm":   {Ecosystem: "npm", Name: "example-package", Version: "1.2.3"},
	}
	client := fixedOSV(func(_ context.Context, queries []domain.OSVQuery) []domain.OSVResult {
		if len(queries) != len(want) {
			t.Fatalf("unexpected queries: %+v", queries)
		}
		results := make([]domain.OSVResult, 0, len(queries))
		for _, query := range queries {
			if query != want[query.Ecosystem] {
				t.Fatalf("unexpected query: %+v", query)
			}
			results = append(results, domain.OSVResult{Query: query, Pages: []domain.OSVPage{
				{VulnerabilityIDs: []string{"SHARED-ID"}, Severity: map[string]string{"SHARED-ID": "critical"}, ObtainedAt: a.AssessmentAt},
			}})
		}
		return results
	})
	out, err := NewProcessorWithSources(engine, nil, client).Process(t.Context(), a, mixedSession{controlSession{git}}, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	metric := out.Report.Categories[3].Metrics[1]
	raw, _ := metric.Input.RawValue.MarshalJSON()
	if string(raw) != "30" || len(out.Snapshot.Vulnerabilities) != 3 {
		t.Fatalf("expected three distinct findings and risk 30, got %s and %+v", raw, out.Snapshot.Vulnerabilities)
	}
}

type secretSession struct{ controlSession }

func (s secretSession) HistoryTree(context.Context, string) ([]domain.GitFile, error) {
	return []domain.GitFile{{Path: "historical.txt", Mode: "100644", ObjectSHA: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Size: 21}}, nil
}
func (s secretSession) ReadBlob(context.Context, string) ([]byte, error) {
	return []byte("token = \"abcdefghijk\""), nil
}

func TestRuntimeProcessorIncludesSecretHistory(t *testing.T) {
	a := runningAnalysis(t)
	e, err := scoring.Default()
	if err != nil {
		t.Fatal(err)
	}
	data := domain.GitData{Repository: a.Repository, Commits: []domain.GitCommit{{SHA: *a.Repository.CommitSHA}}, Sources: []domain.SourceState{
		{Source: "git", Scope: "refs", Status: "complete"}, {Source: "git", Scope: "history", Status: "complete"},
	}}
	output, err := NewProcessor(e).Process(t.Context(), a, secretSession{controlSession{data}}, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	published, err := domain.NewResult(a, domain.NewID(), domain.NewID(), a.AssessmentAt, output.Snapshot, output.Report)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := e.Replay(t.Context(), published.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(published.Report.Categories, replayed.Categories) {
		t.Fatal("secret counts changed during replay")
	}
	input := output.Snapshot.Inputs[6]
	metric := output.Report.Categories[3].Metrics[0]
	if input.ID != "SEC_SECRETS" || input.Status != "complete" || len(input.Evidence) != 1 || metric.NormalizedScore == nil || *metric.NormalizedScore != 50 {
		t.Fatalf("secret scan did not reach runtime snapshot/report: %+v %+v", input, metric)
	}
	if err := output.Snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
}
